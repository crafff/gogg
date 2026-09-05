PYTHON ?= python3

.PHONY: check test knowledge-check foundation-check precommit install-tools

check: foundation-check knowledge-check test

foundation-check:
	$(PYTHON) -B tools/check_foundation.py

knowledge-check:
	$(PYTHON) -B -m tools.agent_system check

test:
	$(PYTHON) -B -m unittest discover -s tests -v

precommit:
	$(PYTHON) -B tools/precommit.py

install-tools:
	$(PYTHON) -B tools/install_gitleaks.py
