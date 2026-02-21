.PHONY: help clean build typecheck test check

help:
	@echo "Targets:"
	@echo "  build      - compile TypeScript to dist/"
	@echo "  typecheck  - run TypeScript checks"
	@echo "  test       - run Node test suite"
	@echo "  check      - typecheck + test"
	@echo "  clean      - remove dist/"

clean:
	pnpm clean

build:
	pnpm build

typecheck:
	pnpm typecheck

test:
	pnpm test

check:
	pnpm check
