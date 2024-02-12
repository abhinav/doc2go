SHELL = /bin/bash
DOC2GO = $(shell pwd)/../bin/doc2go

EMBEDDED_REF = content/en/api/_index.html
STANDALONE_REF = content/en/example/index.html
STD_REF = content/en/std/index.html

DEPS = \
	standalone \
	embedded \
	std \
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
	rm -fr content/en/api/ content/en/example/
	rm -fr public/*

.PHONY: standalone
standalone: $(STANDALONE_REF)

.PHONY: embedded
embedded: $(EMBEDDED_REF)

.PHONY: std
std: $(STD_REF)

$(STD_REF): $(DOC2GO) $(PAGEFIND)
	$(DOC2GO) -pagefind=$(PAGEFIND) -out=content/en/std std

$(STANDALONE_REF): $(DOC2GO) $(PAGEFIND)
	cd .. && $(DOC2GO) -config docs/standalone.rc -pagefind=$(PAGEFIND) ./...

$(EMBEDDED_REF): $(DOC2GO) frontmatter.tmpl
	cd .. && $(DOC2GO) -config docs/embed.rc ./...

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
