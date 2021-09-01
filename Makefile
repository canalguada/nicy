SHELL		= /bin/bash
DESTDIR		?=
package		= nicy
program		= nicy
git_branch	= master
version		= 0.1.6
revision	= 1
release_dir	= build
prefix		= /usr/local
bindir		= $(prefix)/bin
confdir		= $(prefix)/etc/$(program)
libdir		= $(prefix)/lib/$(program)

SUDO		= $$([ $$UID -ne 0 ] && echo "sudo")

.DEFAULT_GOAL := default

.PHONY: build
build:
	/usr/bin/go build -o $(program) -ldflags="-s" .

.PHONY: capabilities
capabilities:
	$(SUDO) /usr/sbin/setcap "cap_setpcap,cap_sys_nice=p" ./$(program)

.PHONY: default
default: build capabilities

.PHONY: all
all:
	: # do nothing

.PHONY: clean
clean:
	: # do nothing

.PHONY: distclean
distclean:
	$(SUDO) find $(release_dir)/ -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} \;

.PHONY: man
man:
	cd man/fragments ; \
	cat HEADERS SYNOPSIS DESCRIPTION COMMANDS OPTIONS FILES \
	CONFIGURATION EXAMPLES VARIABLES DIAGNOSTICS BUGS SEE_ALSO \
	NOTES | \
	sed \
	-e 's#%prefix%#$(prefix)#g' -e 's#%version%#$(version)#g' \
	- > ../$(program).1 ; \
	cd .. ; \
	gzip -9 -f $(program).1

.PHONY: deb
deb:
	version=$(version) \
	revision=$(revision) \
	release_dir=$(release_dir) \
	./package.sh

.PHONY: dist
dist:
	mkdir -p $(release_dir)
	git archive --format=tar.gz \
		-o $(release_dir)/$(package)-$(version).tar.gz \
		--prefix=$(package)-$(version)/ \
		$(git_branch)

.PHONY: install-bin
install-bin:
	install -d $(DESTDIR)$(bindir)
	install -m755 $(program) $(DESTDIR)$(bindir)/$(program)

.PHONY: install-man
install-man: man
	install -d $(DESTDIR)$(prefix)/share/man/man1
	install -m644 man/$(program).1.gz $(DESTDIR)$(prefix)/share/man/man1/

.PHONY: install-lib
install-lib:
	install -d $(DESTDIR)$(libdir)/jq
	install -m644 lib/jq/common.jq $(DESTDIR)$(libdir)/jq/
	install -m644 lib/jq/install.jq $(DESTDIR)$(libdir)/jq/
	install -m644 lib/jq/list.jq $(DESTDIR)$(libdir)/jq/
	install -m644 lib/jq/manage.jq $(DESTDIR)$(libdir)/jq/
	install -m644 lib/jq/rebuild.jq $(DESTDIR)$(libdir)/jq/
	install -m644 lib/jq/run.jq $(DESTDIR)$(libdir)/jq/

.PHONY: install-conf
install-conf:
	install -d $(DESTDIR)$(confdir)/rules.d
	install -m644 conf/config.yaml $(DESTDIR)$(confdir)/
	install -m644 conf/00-cgroups.cgroups $(DESTDIR)$(confdir)/
	install -m644 conf/00-types.types $(DESTDIR)$(confdir)/
	install -m644 conf/rules.d/vim.rules $(DESTDIR)$(confdir)/rules.d/


.PHONY: install
install: install-bin install-man install-lib install-conf

.PHONY: uninstall-bin
uninstall-bin:
	rm -f $(DESTDIR)$(bindir)/$(program)

.PHONY: uninstall-man
uninstall-man:
	rm -f $(DESTDIR)$(prefix)/share/man/man1/$(program).1.gz

.PHONY: uninstall-lib
uninstall-lib:
	rm -rf $(DESTDIR)$(libdir)

.PHONY: uninstall-conf
uninstall-conf:
	rm -f $(DESTDIR)$(confdir)/environment
	rm -f $(DESTDIR)$(confdir)/00-cgroups.cgroups
	rm -f $(DESTDIR)$(confdir)/00-types.types
	rm -f $(DESTDIR)$(confdir)/rules.d/vim.rules
	rmdir --ignore-fail-on-non-empty $(DESTDIR)$(confdir)/rules.d
	rmdir --ignore-fail-on-non-empty $(DESTDIR)$(confdir)

.PHONY: uninstall
uninstall: uninstall-bin uninstall-man uninstall-lib uninstall-conf

