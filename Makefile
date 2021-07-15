SHELL		= /bin/bash
DESTDIR		?=
package		= nicy
program		= nicy
git_branch	= golang
version		= 0.1.5
revision	= 1
release_dir	= ..
release_file	= $(release_dir)/$(package)-$(version)
prefix		= /usr/local
bindir		= $(prefix)/bin
confdir		= $(prefix)/etc/$(program)
libdir		= $(prefix)/lib/$(program)


.DEFAULT_GOAL := build

build:
	/usr/bin/go build -o $(program) -ldflags="-s" .

capabilities:
	/usr/sbin/setcap "cap_setpcap,cap_sys_nice=p" ./$(program)
all:
	: # do nothing

clean:
	: # do nothing

distclean: clean

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

# deb: dist
	# cd $(release_dir) && tar -xf $(package)-$(version).tar.gz && \
	# cd $(package)-$(version) && \
	# debmake -b"$(package):bin" -u"$(version)" -r"$(revision)" && \
	# debuild
	# [WIP] Build with dh-make-golang make -force-prerelease  -git_revision $(git_branch) -type p github.com/canalguada/nicy

dist:
	git archive --format=tar.gz \
		-o $(release_dir)/$(package)-$(version).tar.gz \
		--prefix=$(package)-$(version)/ \
		$(git_branch)

install-bin:
	install -d $(DESTDIR)$(bindir)
	install -m755 $(program) $(DESTDIR)$(bindir)/$(program)

install-man: man
	install -d $(DESTDIR)$(prefix)/share/man/man1
	install -m644 man/$(program).1.gz $(DESTDIR)$(prefix)/share/man/man1/

install-lib:
	install -d $(DESTDIR)$(libdir)/jq
	install -m644 lib/jq/common.jq $(DESTDIR)$(libdir)/jq/
	install -m644 lib/jq/install.jq $(DESTDIR)$(libdir)/jq/
	install -m644 lib/jq/list.jq $(DESTDIR)$(libdir)/jq/
	install -m644 lib/jq/manage.jq $(DESTDIR)$(libdir)/jq/
	install -m644 lib/jq/rebuild.jq $(DESTDIR)$(libdir)/jq/
	install -m644 lib/jq/run.jq $(DESTDIR)$(libdir)/jq/

install-conf:
	install -d $(DESTDIR)$(confdir)/rules.d
	install -m644 conf/environment $(DESTDIR)$(confdir)/
	sed -i 's#%prefix%#$(prefix)#g' $(DESTDIR)$(confdir)/environment
	install -m644 conf/00-cgroups.cgroups $(DESTDIR)$(confdir)/
	install -m644 conf/00-types.types $(DESTDIR)$(confdir)/
	install -m644 conf/rules.d/vim.rules $(DESTDIR)$(confdir)/rules.d/


install: build capabilities install-bin install-man install-lib install-conf

uninstall-bin:
	rm -f $(DESTDIR)$(bindir)/$(program)

uninstall-man:
	rm -f $(DESTDIR)$(prefix)/share/man/man1/$(program).1.gz

uninstall-lib:
	rm -rf $(DESTDIR)$(libdir)

uninstall-conf:
	rm -f $(DESTDIR)$(confdir)/environment
	rm -f $(DESTDIR)$(confdir)/00-cgroups.cgroups
	rm -f $(DESTDIR)$(confdir)/00-types.types
	rm -f $(DESTDIR)$(confdir)/rules.d/vim.rules
	rmdir --ignore-fail-on-non-empty $(DESTDIR)$(confdir)/rules.d
	rmdir --ignore-fail-on-non-empty $(DESTDIR)$(confdir)

uninstall: uninstall-bin uninstall-man uninstall-lib uninstall-conf

.PHONY: all build clean distclean man deb dist \
	install-bin install-man install-lib install-conf \
	uninstall-bin uninstall-man uninstall-lib uninstall-conf \
	install uninstall
