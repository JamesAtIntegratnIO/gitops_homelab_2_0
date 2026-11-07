# Hermes Agent, plus local speech-to-text (faster-whisper).
#
# The upstream image bakes a fixed set of Python extras into a sealed venv and
# leaves the rest to lazy install at run time. We run with
# security.allow_lazy_installs: false, and the venv is root-owned anyway (the
# agent runs as uid 10000), so local STT has to be built in.
#
# Upstream's /opt/hermes, its venv and its package-manager state are left
# exactly as shipped. Rebuilding the venv instead (the first version of this
# file) dropped Hermes's source binding and put PM's install records out of
# sync, so the gateway crash-looped at boot (2026-11-07, reverted in #676).
#
# Instead:
#   1. Export the stt-whisper extra's requirements from Hermes's own uv.lock
#      (exact pins: faster-whisper 1.2.1, ctranslate2 4.8.3, ...).
#   2. Drop every package the venv already has, so nothing shadows it.
#   3. Install the rest, without resolving (--no-deps: the export is the full
#      closure), into /opt/hermes-stt with upstream's bundled uv.
#   4. Put /opt/hermes-stt on PYTHONPATH. Hermes's launcher runs python with
#      -P only, which honours PYTHONPATH.
#
# The base is pinned by digest; bump it together with the tag in the hermes
# Deployment's comments (addons/cluster-roles/control-plane/addons/hermes/).
FROM docker.io/nousresearch/hermes-agent:v0.23.0@sha256:adb7890916de63245523492be8f68f9d2095fecbec4e2a6dab7960381f0fc002

USER root
WORKDIR /opt/hermes

# HERMES_HOME is /opt/data in the base image. Point it at a throwaway dir for
# this step: running Hermes code at build time otherwise creates root-owned
# directories in /opt/data, which then seed the data volume and make the boot
# fail with "Permission denied" on /opt/data/logs.
RUN set -eu; \
    export HERMES_HOME=/tmp/hermes-build-home; mkdir -p "$HERMES_HOME"; \
    python3 -m pm.build_env --source /opt/hermes --python /usr/local/bin/python3 \
      --no-install-project --extra stt-whisper --export-requirements /tmp/stt-all.txt; \
    /opt/hermes/.venv/bin/python -c 'import re, sys; from importlib.metadata import distributions; norm = lambda n: re.sub(r"[-_.]+", "-", n).lower(); have = {norm(d.metadata["Name"]) for d in distributions()}; keep = [l for l in open(sys.argv[1]) if l.strip() and not l.startswith(("#", " ")) and norm(l.split("==")[0]) not in have]; open(sys.argv[2], "w").writelines(keep); print("stt packages to add:", len(keep))' /tmp/stt-all.txt /tmp/stt.txt; \
    uv=$(ls /opt/hermes/tools/uv-*/uv); \
    "$uv" pip install --no-deps --no-cache --target /opt/hermes-stt \
      --python /opt/hermes/.venv/bin/python -r /tmp/stt.txt; \
    rm -rf /tmp/stt-all.txt /tmp/stt.txt; \
    chmod -R a+rX,go-w /opt/hermes-stt; \
    PYTHONPATH=/opt/hermes-stt /opt/hermes/.venv/bin/python -c "import faster_whisper, ctranslate2, av, onnxruntime, hermes_cli; print('faster-whisper', faster_whisper.__version__)"; \
    PYTHONPATH=/opt/hermes-stt /opt/hermes/.venv/bin/hermes --version; \
    rm -rf "$HERMES_HOME" /root/.cache

ENV PYTHONPATH=/opt/hermes-stt

# Upstream's ENTRYPOINT (s6-overlay via entrypoint-dispatch.sh), CMD, USER and
# WORKDIR are inherited unchanged.
