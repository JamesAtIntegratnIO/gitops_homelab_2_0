{{/*
kargo-pipelines helpers.

Every Kargo expression is assembled here as a Go string and emitted with
`quote`, so the `${{ ... }}` delimiters never appear as raw template text
(Helm would otherwise try to parse the inner `{{ }}` itself).
*/}}

{{- /* The name label is a value so a repository migrating from an earlier
     chart can keep its existing one. Changing it rewrites the label on every
     rendered object, which ArgoCD will happily do -- but it is churn nobody
     asked for in the middle of a migration. */ -}}
{{- define "kp.labels" -}}
app.kubernetes.io/name: {{ .Values.nameLabel | default "kargo-pipelines" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: kargo
{{- end -}}

{{/* Wrap an expr-lang expression in Kargo's delimiters. */}}
{{- define "kp.expr" -}}
{{- printf "${{ %s }}" . -}}
{{- end -}}

{{/*
Validate a target and return a normalised dict:
  isImage, strategy, semver (bool: versions are semver-comparable),
  format, autoMerge, autoPromotion, interval, discoveryLimit, scope
Called with (dict "root" $ "project" <name> "name" <target name> "target" <spec>).
*/}}
{{- define "kp.normalise" -}}
{{- $root := .root -}}{{- $t := .target -}}{{- $name := .name -}}{{- $d := $root.Values.defaults -}}
{{- $kinds := 0 -}}{{- range (list "image" "chart" "git") -}}{{- if hasKey $t . -}}{{- $kinds = add1 $kinds -}}{{- end -}}{{- end -}}
{{- if ne (int $kinds) 1 -}}{{- fail (printf "kargo-pipelines: target %q needs exactly one of image, chart, git" $name) -}}{{- end -}}
{{- /* Either the target carries updates directly (one implicit stage), or it
     declares stages and each of those carries its own. A stage with no
     updates would render a promotion pipeline that reads nothing and
     rewrites nothing, and would look like it was working. */ -}}
{{- if $t.stages -}}
  {{- range $i, $st := $t.stages -}}
    {{- if not $st.name -}}{{- fail (printf "kargo-pipelines: target %q: stages[%d] has no name" $name $i) -}}{{- end -}}
    {{- if not $st.updates -}}{{- fail (printf "kargo-pipelines: target %q: stage %q has no updates" $name $st.name) -}}{{- end -}}
  {{- end -}}
  {{- if lt (len $t.stages) 2 -}}{{- fail (printf "kargo-pipelines: target %q declares a single stage; omit `stages` and put updates on the target instead" $name) -}}{{- end -}}
{{- else if not $t.updates -}}
  {{- fail (printf "kargo-pipelines: target %q has no updates" $name) -}}
{{- end -}}
{{- $isImage := not (empty $t.image) -}}
{{- $strategy := "SemVer" -}}
{{- if $isImage -}}{{- $strategy = default "SemVer" $t.image.strategy -}}{{- end -}}
{{- if not (has $strategy (list "SemVer" "Digest" "NewestBuild" "Lexical")) -}}{{- fail (printf "kargo-pipelines: target %q: unknown strategy %q" $name $strategy) -}}{{- end -}}
{{- if and (eq $strategy "Digest") (not $t.image.constraint) -}}{{- fail (printf "kargo-pipelines: target %q: Digest strategy needs constraint (the mutable tag)" $name) -}}{{- end -}}
{{- $isGit := not (empty $t.git) -}}
{{- $semver := or (not $isImage) (eq $strategy "SemVer") -}}
{{- $format := default (ternary "image" (ternary "tag" "version" $isGit) $isImage) $t.format -}}
{{- $autoMerge := default $d.autoMerge $t.autoMerge -}}
{{- if not (has $autoMerge (list "always" "minor" "patch" "never")) -}}{{- fail (printf "kargo-pipelines: target %q: unknown autoMerge %q" $name $autoMerge) -}}{{- end -}}
{{- $kind := ternary (ternary "build" "image" (eq $strategy "NewestBuild")) "chart" $isImage -}}
{{- $interval := default (index $d.interval $kind) $t.interval -}}
{{- $limit := index $d.discoveryLimit $kind -}}
{{- if $isImage -}}{{- $limit = default $limit $t.image.discoveryLimit -}}{{- else if $isGit -}}{{- $limit = default $limit $t.git.discoveryLimit -}}{{- else -}}{{- $limit = default $limit $t.chart.discoveryLimit -}}{{- end -}}
{{- $autoPromotion := $d.autoPromotion -}}
{{- if hasKey $t "autoPromotion" -}}{{- $autoPromotion = $t.autoPromotion -}}{{- end -}}
{{- dict "isImage" $isImage "isGit" $isGit "strategy" $strategy "semver" $semver "format" $format "autoMerge" $autoMerge "autoPromotion" $autoPromotion "interval" $interval "discoveryLimit" $limit "scope" (default .project $t.scope) | toJson -}}
{{- end -}}

{{/* expr-lang call that yields the subscribed artifact object. */}}
{{- define "kp.artifactFn" -}}
{{- if .image -}}
imageFrom({{ .image.repoURL | quote }})
{{- else if .git -}}
commitFrom({{ .git.repoURL | quote }})
{{- else if .chart.name -}}
chartFrom({{ .chart.repoURL | quote }}, {{ .chart.name | quote }})
{{- else -}}
chartFrom({{ .chart.repoURL | quote }})
{{- end -}}
{{- end -}}

{{/* Version-ish string of the new artifact, for semverDiff. (target) */}}
{{- define "kp.newVersionExpr" -}}
{{- $fn := include "kp.artifactFn" . -}}
{{- if .image -}}
{{- if eq (default "SemVer" .image.strategy) "Digest" -}}{{ $fn }}.Digest{{- else -}}{{ $fn }}.Tag{{- end -}}
{{- else if .git -}}{{ $fn }}.Tag
{{- else -}}{{ $fn }}.Version{{- end -}}
{{- end -}}

{{/* Short, human-readable form of the new version for titles and messages. (target) */}}
{{- define "kp.newVersionDisplayExpr" -}}
{{- $fn := include "kp.artifactFn" . -}}
{{- if and .image (eq (default "SemVer" .image.strategy) "Digest") -}}
{{ printf "%q + %s.Digest[:19]" (printf "%s@" .image.constraint) $fn }}
{{- else -}}{{ include "kp.newVersionExpr" . }}{{- end -}}
{{- end -}}

{{/* What gets written into the file, as one expr-lang expression. (dict target format) */}}
{{- define "kp.newValueExpr" -}}
{{- $t := .target -}}{{- $fn := include "kp.artifactFn" $t -}}
{{- if eq .format "image" -}}
{{ printf "%q + %s.Tag + \"@\" + %s.Digest" (printf "%s:" $t.image.repoURL) $fn $fn }}
{{- else if eq .format "image-tag" -}}
{{ printf "%q + %s.Tag" (printf "%s:" $t.image.repoURL) $fn }}
{{- else if eq .format "tag" -}}
{{ printf "%s.Tag" $fn }}
{{- else if eq .format "version" -}}
{{ printf (ternary "%s.Tag" "%s.Version" (not (empty $t.git))) $fn }}
{{- else -}}
{{- fail (printf "kargo-pipelines: unknown format %q" .format) -}}
{{- end -}}
{{- end -}}

{{/*
Turn an sjson-style key (a.b.0.c) into an expr-lang path over the parsed
document (a.b[0].c). Segments that are not identifiers are bracketed; at the
top level that means going through `$env`, the expression environment.
*/}}
{{- define "kp.exprPath" -}}
{{- $out := "" -}}
{{- range $i, $seg := splitList "." . -}}
{{- if regexMatch "^[0-9]+$" $seg -}}
{{- $out = printf "%s[%s]" $out $seg -}}
{{- else if regexMatch "^[A-Za-z_][A-Za-z0-9_]*$" $seg -}}
{{- if eq $i 0 -}}{{- $out = $seg -}}{{- else -}}{{- $out = printf "%s.%s" $out $seg -}}{{- end -}}
{{- else -}}
{{- if eq $i 0 -}}{{- $out = printf "$env[%q]" $seg -}}{{- else -}}{{- $out = printf "%s[%q]" $out $seg -}}{{- end -}}
{{- end -}}
{{- end -}}
{{- $out -}}
{{- end -}}

{{/* Expression extracting the comparable version from the current value. (dict path format) */}}
{{- define "kp.currentVersionExpr" -}}
{{- if has .format (list "image" "image-tag") -}}
{{ printf "last(split(split(%s, \"@\")[0], \":\"))" .path }}
{{- else -}}{{ .path }}{{- end -}}
{{- end -}}

{{/* "Is there anything to do?" condition. (dict target norm) */}}
{{- define "kp.changedExpr" -}}
{{- /* Reads outputs.read.*, which the stage's own yaml-parse produced from the
     first file of ITS updates -- so per-stage correctness needs nothing here. */ -}}
{{- $n := .norm -}}
{{- if $n.semver -}}
{{ printf "semverDiff(%s, outputs.read.currentVersion) != \"None\"" (include "kp.newVersionExpr" .target) }}
{{- else -}}
{{ printf "outputs.read.current != %s" (include "kp.newValueExpr" (dict "target" .target "format" $n.format)) }}
{{- end -}}
{{- end -}}

{{/* "May the Stage merge this itself?" condition. (dict target norm) */}}
{{- define "kp.autoMergeExpr" -}}
{{- $n := .norm -}}
{{- /* A stage may tighten the target's policy -- a canary can merge itself
     while the stage that reaches production still waits for a human. */ -}}
{{- $policy := default $n.autoMerge .autoMerge -}}
{{- $new := include "kp.newVersionExpr" .target -}}
{{- $diff := printf "semverDiff(%s, outputs.read.currentVersion)" $new -}}
{{- if eq $policy "always" -}}true
{{- else if eq $policy "never" -}}false
{{- else if not $n.semver -}}false
{{- else if eq $policy "patch" -}}
{{ printf "(%s in [\"Patch\", \"Metadata\"])" $diff }}
{{- else -}}
{{ printf "((%s in [\"Patch\", \"Metadata\"]) || (%s == \"Minor\" && semverParse(%s).Major() > 0))" $diff $diff $new }}
{{- end -}}
{{- end -}}

{{/* Conventional-commit subject used for the commit and the PR title. (dict target name norm) */}}
{{- define "kp.subject" -}}
{{ printf "chore(%s): bump %s to ${{ %s }}" .norm.scope .name (include "kp.newVersionDisplayExpr" .target) }}
{{- end -}}

{{/*
kp.stageList -- normalise a target into an ordered list of stages.

A target with no `stages:` is one implicit stage taking the target's own
`updates`/`verify`/`autoMerge`, which is the single-environment shape and stays
the default.

With `stages:`, the list order IS the promotion order, and the LAST entry keeps
the target's bare name. That is deliberate: renaming the terminal Stage would
discard its freight and verification history and make ArgoCD prune and recreate
it, so a repo adopting a chain keeps the object it already had and gains a new
upstream one. See docs/chaining.md.
*/}}
{{- define "kp.stageList" -}}
{{- $target := .target -}}
{{- $name := .name -}}
{{- $out := list -}}
{{- if $target.stages -}}
  {{- $count := len $target.stages -}}
  {{- range $i, $s := $target.stages -}}
    {{- $isLast := eq (add1 $i) $count -}}
    {{- $stageName := $name -}}
    {{- if not $isLast -}}{{- $stageName = printf "%s-%s" $name $s.name -}}{{- end -}}
    {{- /* `ternary` evaluates BOTH arms, so the upstream lookup has to sit
         behind a real conditional -- index stages -1 is a hard error. */ -}}
    {{- $upstream := "" -}}
    {{- $soak := "" -}}
    {{- if gt $i 0 -}}
      {{- $prev := index $target.stages (sub $i 1) -}}
      {{- $upstream = printf "%s-%s" $name $prev.name -}}
      {{- /* Kargo puts requiredSoakTime on the DOWNSTREAM stage's sources, but
           an author naturally writes "soak here for 30m" on the stage doing the
           soaking. Read it from the upstream declaration and render it
           downstream, so the values say what people mean. */ -}}
      {{- $soak = default "" $prev.requiredSoakTime -}}
    {{- end -}}
    {{- $out = append $out (dict
          "stageName" $stageName
          "key" $s.name
          "index" $i
          "isFirst" (eq $i 0)
          "isLast" $isLast
          "updates" $s.updates
          "verify" (default $target.verify $s.verify)
          "autoMerge" (default $target.autoMerge $s.autoMerge)
          "autoPromotion" $s.autoPromotion
          "requiredSoakTime" $soak
          "upstream" $upstream
        ) -}}
  {{- end -}}
{{- else -}}
  {{- $out = append $out (dict
        "stageName" $name
        "key" ""
        "index" 0
        "isFirst" true
        "isLast" true
        "updates" $target.updates
        "verify" $target.verify
        "autoMerge" $target.autoMerge
        "autoPromotion" nil
        "requiredSoakTime" ""
        "upstream" ""
      ) -}}
{{- end -}}
{{- $out | toJson -}}
{{- end -}}

{{/*
kp.triageBody -- the JSON payload handed to the triage service.

Assembled from strings, like the PR description, so Kargo's ${{ }} expressions
never meet Helm's parser.
*/}}
{{- define "kp.triageBody" -}}
{{- $t := .target -}}
{{- $n := .norm -}}
{{- $pairs := list
      (printf "\"project\": %s" (include "kp.expr" "quote(ctx.project)"))
      (printf "\"stage\": %s" (include "kp.expr" "quote(ctx.stage)"))
      (printf "\"promotion\": %s" (include "kp.expr" "quote(ctx.promotion)"))
      (printf "\"artifact\": %s" (.artifact | quote))
      (printf "\"from\": %s" (include "kp.expr" "quote(outputs.read.currentVersion)"))
      (printf "\"to\": %s" (include "kp.expr" (printf "quote(%s)" (include "kp.newVersionDisplayExpr" $t))))
      (printf "\"autoMerge\": %s" ($n.autoMerge | quote))
      (printf "\"prNumber\": %s" (include "kp.expr" "outputs.pr.pr.id"))
      (printf "\"prURL\": %s" (include "kp.expr" "quote(outputs.pr.pr.url)"))
      (printf "\"branch\": %s" (include "kp.expr" "quote(outputs.push.branch)"))
      (printf "\"files\": %s" (.files | toJson))
      (printf "\"verifyApps\": %s" (.verifyApps | toJson)) -}}
{{- printf "{%s}" (join ", " $pairs) -}}
{{- end -}}

{{/* kp.stageFiles -- the files one stage touches, for the triage payload. */}}
{{- define "kp.stageFiles" -}}
{{- $files := list -}}
{{- range $u := .stage.updates -}}
{{- $files = append $files $u.file -}}
{{- end -}}
{{- $files | toJson -}}
{{- end -}}
