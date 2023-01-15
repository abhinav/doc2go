SHELL = /bin/bash
DOC2GO = $(shell pwd)/../bin/doc2go

EMBEDDED_REF = content/en/api/_index.html
STANDALONE_REF = content/en/example/index.html

DEPS = \
	standalone \
	embedded \
	content/en/docs/usage/usage.txt

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

$(STANDALONE_REF): $(DOC2GO)
	cd .. && $(DOC2GO) \
		-internal \
		-home go.abhg.dev/doc2go \
		-out docs/content/en/example ./...

$(EMBEDDED_REF): $(DOC2GO) frontmatter.tmpl
	cd .. && $(DOC2GO) \
		-basename _index.html -embed -internal \
		-frontmatter docs/frontmatter.tmpl \
		-home go.abhg.dev/doc2go \
		-out docs/content/en/api ./...

content/en/docs/usage/usage.txt: $(DOC2GO)
	$(DOC2GO) -h &> $@

$(DOC2GO): doc2go.force
	make -C .. bin/doc2go

.PHONY: doc2go.force
doc2go.force:
