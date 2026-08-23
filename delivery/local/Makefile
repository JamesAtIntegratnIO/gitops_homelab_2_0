# The local proving ground. See README.md.
#
# LLM_BASE_URL has no default on purpose: the agent talks to a model endpoint
# and guessing one for you is how a demo quietly starts spending money.

SHELL := /usr/bin/env bash
SCRIPTS := scripts

.PHONY: help
help:
	@echo "make up      colima + idpbuilder + platform + kit + seed  (everything)"
	@echo "make demo    run the whole flow, asserted at every step"
	@echo "make demo-triage  a PR the gate REFUSES, and the agent's verdict on it"
	@echo "make reset   put the flow back to the start so demo can run again"
	@echo "make down    delete the cluster; colima keeps running"
	@echo "make clean   down, and stop colima too"
	@echo
	@echo "required:  LLM_BASE_URL=http://<host>:<port>/v1"
	@echo "optional:  LLM_MODEL (default qwen/qwen3.5-9b)"

.PHONY: up
up: runtime cluster platform seed kit
	@echo "ready -- now run: make demo"

.PHONY: runtime
runtime:
	@bash $(SCRIPTS)/00-runtime.sh

.PHONY: cluster
cluster:
	@bash $(SCRIPTS)/05-cluster.sh

.PHONY: platform
platform:
	@bash $(SCRIPTS)/10-platform.sh

.PHONY: seed
seed:
	@bash $(SCRIPTS)/20-seed.sh

.PHONY: kit
kit:
	@bash $(SCRIPTS)/30-kit.sh

.PHONY: demo
demo:
	@bash $(SCRIPTS)/40-demo.sh

# The demo consumes the Freight it promotes, so a second run has nothing left
# to do. This puts BOTH sides back -- the repository and Kargo's stage state.
# Act two: a pull request the gate refuses, and an agent that has to answer
# for it. Run after `make demo`.
.PHONY: demo-triage
demo-triage:
	@bash $(SCRIPTS)/50-demo-triage.sh


.PHONY: reset
reset:
	@bash $(SCRIPTS)/95-reset.sh

.PHONY: down
down:
	@bash $(SCRIPTS)/90-down.sh

.PHONY: clean
clean: down
	@colima stop
