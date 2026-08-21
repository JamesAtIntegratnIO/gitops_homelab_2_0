{{/*
kargo-projects helpers.

Every Kargo expression is assembled here as a Go string and emitted with
`quote`, so the `${{ ... }}` delimiters never appear as raw template text
(Helm would otherwise try to parse the inner `{{ }}` itself).
*/}}

{{- define "kp.labels" -}}
app.kubernetes.io/name: kargo-projects
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
{{- if ne (int $kinds) 1 -}}{{- fail (printf "kargo-projects: target %q needs exactly one of image, chart, git" $name) -}}{{- end -}}
{{- if not $t.updates -}}{{- fail (printf "kargo-projects: target %q has no updates" $name) -}}{{- end -}}
{{- $isImage := not (empty $t.image) -}}
{{- $strategy := "SemVer" -}}
{{- if $isImage -}}{{- $strategy = default "SemVer" $t.image.strategy -}}{{- end -}}
{{- if not (has $strategy (list "SemVer" "Digest" "NewestBuild" "Lexical")) -}}{{- fail (printf "kargo-projects: target %q: unknown strategy %q" $name $strategy) -}}{{- end -}}
{{- if and (eq $strategy "Digest") (not $t.image.constraint) -}}{{- fail (printf "kargo-projects: target %q: Digest strategy needs constraint (the mutable tag)" $name) -}}{{- end -}}
{{- $isGit := not (empty $t.git) -}}
{{- $semver := or (not $isImage) (eq $strategy "SemVer") -}}
{{- $format := default (ternary "image" (ternary "tag" "version" $isGit) $isImage) $t.format -}}
{{- $autoMerge := default $d.autoMerge $t.autoMerge -}}
{{- if not (has $autoMerge (list "always" "minor" "patch" "never")) -}}{{- fail (printf "kargo-projects: target %q: unknown autoMerge %q" $name $autoMerge) -}}{{- end -}}
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
{{- fail (printf "kargo-projects: unknown format %q" .format) -}}
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
{{- $new := include "kp.newVersionExpr" .target -}}
{{- $diff := printf "semverDiff(%s, outputs.read.currentVersion)" $new -}}
{{- if eq $n.autoMerge "always" -}}true
{{- else if eq $n.autoMerge "never" -}}false
{{- else if not $n.semver -}}false
{{- else if eq $n.autoMerge "patch" -}}
{{ printf "(%s in [\"Patch\", \"Metadata\"])" $diff }}
{{- else -}}
{{ printf "((%s in [\"Patch\", \"Metadata\"]) || (%s == \"Minor\" && semverParse(%s).Major() > 0))" $diff $diff $new }}
{{- end -}}
{{- end -}}

{{/* Conventional-commit subject used for the commit and the PR title. (dict target name norm) */}}
{{- define "kp.subject" -}}
{{ printf "chore(%s): bump %s to ${{ %s }}" .norm.scope .name (include "kp.newVersionDisplayExpr" .target) }}
{{- end -}}
