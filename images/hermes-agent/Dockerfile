# Hermes Agent, plus local speech-to-text (faster-whisper).
#
# The upstream image bakes a fixed set of Python extras into a sealed venv and
# leaves the rest to lazy install at run time. We run with
# security.allow_lazy_installs: false, and the venv is root-owned anyway (the
# agent runs as uid 10000), so local STT has to be built in.
#
# This rebuilds the venv with Hermes's own build command (the one in upstream's
# Dockerfile at v0.23.0) and its own uv.lock, with one extra added:
# stt-whisper (faster-whisper 1.2.1, ctranslate2 4.8.3, ...). Every version
# comes from the lock in the base image, so a change to the FROM line is the
# only way anything here moves. --record-selection makes the extra part of the
# image's recorded baseline, so Hermes's boot-time dependency refresh keeps it.
#
# The base is pinned by digest; bump it together with the tag in the hermes
# Deployment's comments (addons/cluster-roles/control-plane/addons/hermes/).
FROM docker.io/nousresearch/hermes-agent:v0.23.0@sha256:adb7890916de63245523492be8f68f9d2095fecbec4e2a6dab7960381f0fc002

USER root
WORKDIR /opt/hermes

RUN mv /opt/hermes/.venv /opt/hermes/.venv-base \
 && python3 -m pm.build_env --source /opt/hermes --python /usr/local/bin/python3 \
      --cache /opt/hermes/uv-cache \
      --out /opt/hermes/.venv --no-install-project --sealed --record-selection \
      --extra all --extra messaging --extra otlp --extra anthropic --extra bedrock \
      --extra azure-identity --extra matrix --extra google-chat --extra fal --extra documents \
      --extra stt-whisper \
 && /opt/hermes/.venv/bin/python -c "import faster_whisper, ctranslate2, aiohttp, anthropic" \
 && rm -rf /opt/hermes/.venv-base

# Upstream's ENTRYPOINT (s6-overlay via entrypoint-dispatch.sh), CMD, USER and
# WORKDIR are inherited unchanged.
