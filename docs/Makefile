SHELL = /bin/bash
DOC2GO = $(shell pwd)/../bin/doc2go

DOC2GO_EMBED_ROOT = content/en/api/_index.html
TRICKS_EMBED_ROOT = content/en/tricks/_index.html
DOC2GO_STANDALONE_ROOT = static/example/index.html
STD_STANDALONE_ROOT = static/std/index.html

DEPS = \
       $(DOC2GO_STANDALONE_ROOT) \
       $(DOC2GO_EMBED_ROOT) \
       $(STD_STANDALONE_ROOT) \
       $(TRICKS_EMBED_ROOT) \
       assets/scss/_chroma.scss \
       content/en/docs/usage/config-keys.txt

PAGEFIND = $(shell pwd)/node_modules/.bin/pagefind

.PHONY: build
build: $(DEPS)
	hugo --minify

.PHONY: serve
serve: $(DEPS)
	hugo serve --disableLiveReload

.PHONY: clean
clean:
	rm -fr content/en/api/ content/en/example/ content/en/std/ content/en/tricks/
	rm -fr public/*

$(STD_STANDALONE_ROOT): $(DOC2GO) $(PAGEFIND)
	$(DOC2GO) -pagefind=$(PAGEFIND) -out=static/std std

$(DOC2GO_STANDALONE_ROOT): $(DOC2GO) $(PAGEFIND)
	cd .. && $(DOC2GO) -config docs/standalone.rc -pagefind=$(PAGEFIND) ./...

$(DOC2GO_EMBED_ROOT): $(DOC2GO) frontmatter.tmpl
	cd .. && $(DOC2GO) -config docs/embed.rc ./...

$(TRICKS_EMBED_ROOT): $(DOC2GO) frontmatter.tmpl
	cd .. && $(DOC2GO) -config docs/tricks.rc github.com/fluhus/godoc-tricks

assets/scss/_chroma.scss: $(DOC2GO)
	$(DOC2GO) -highlight=tango -highlight-print-css > $@

content/en/docs/usage/config-keys.txt: $(DOC2GO)
	$(DOC2GO) -print-config-keys > $@

$(DOC2GO): doc2go.force
	make -C .. bin/doc2go

$(PAGEFIND): yarn.lock
	yarn install

.PHONY: doc2go.force
doc2go.force:
