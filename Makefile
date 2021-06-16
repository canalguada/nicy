SHELL		= /bin/bash
DESTDIR		?=
package		= nicy
version		= 0.1.4
revision	= 2
release_dir	= ..
release_file	= $(release_dir)/$(package)-$(version)
prefix		= /usr/local
bindir		= $(prefix)/bin
confdir		= $(prefix)/etc/nicy
datadir		= $(prefix)/share/nicy
libdir		= $(prefix)/lib/nicy
jqversion	= .version |= "$(version)"

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
	- > ../nicy.1 ; \
	cd .. ; \
	gzip -9 -f nicy.1

deb:
	debmake -b"nicy:sh" -u"$(version)" -r"$(revision)" -t && \
	cd ../nicy-$(version) && \
	debuild

dist:
	git archive --format=tar.gz -o $(release_dir)/$(package)-$(version).tar.gz --prefix=$(package)-$(version)/ master

install-scripts:
	install -d $(DESTDIR)$(bindir)
	install -m755 scripts/nicy $(DESTDIR)$(bindir)/
	sed -i 's#prefix=/usr/local#prefix=$(prefix)#g' $(DESTDIR)$(bindir)/nicy

install-man: man
	install -d $(DESTDIR)$(prefix)/share/man/man1
	install -m644 man/nicy.1.gz $(DESTDIR)$(prefix)/share/man/man1/

install-lib:
	install -d $(DESTDIR)$(libdir)/jq
	find lib/jq/ -type f -iname "*.jq" \
		-exec install -m644 {} $(DESTDIR)$(libdir)/jq/ \;
	install -m644 lib/procstat.awk $(DESTDIR)$(libdir)/

install-data:
	install -d $(DESTDIR)$(datadir)
	jq -r '$(jqversion)' \
		data/usage.json > $(DESTDIR)$(datadir)/usage.json
	jq -r -L lib/jq \
		'include "usage"; $(jqversion) | main' \
		data/usage.json | groff -T utf8 > $(DESTDIR)$(datadir)/nicy.help
	for func in run show list install rebuild manage; do \
		jqscript=$$(printf 'include "usage"; $(jqversion) | %s' $${func}) ; \
		jq -r -L lib/jq "$${jqscript}" \
			data/usage.json | groff -T utf8 > $(DESTDIR)$(datadir)/$${func}.help ; \
	done

install-conf:
	install -d $(DESTDIR)$(confdir)/rules.d
	install -m644 conf/environment $(DESTDIR)$(confdir)/
	sed -i 's#%prefix%#$(prefix)#g' $(DESTDIR)$(confdir)/environment
	install -m644 conf/00-cgroups.cgroups $(DESTDIR)$(confdir)/
	install -m644 conf/00-types.types $(DESTDIR)$(confdir)/
	install -m644 conf/rules.d/vim.rules $(DESTDIR)$(confdir)/rules.d/


install: install-scripts install-man install-lib install-data install-conf

uninstall-scripts:
	rm -f $(DESTDIR)$(bindir)/nicy

uninstall-man:
	rm -f $(DESTDIR)$(prefix)/share/man/man1/nicy.1.gz

uninstall-data:
	rm -rf $(DESTDIR)$(datadir)

uninstall-lib:
	rm -rf $(DESTDIR)$(libdir)

uninstall-conf:
	rm -f $(DESTDIR)$(confdir)/environment
	rm -f $(DESTDIR)$(confdir)/00-cgroups.cgroups
	rm -f $(DESTDIR)$(confdir)/00-types.types
	rm -f $(DESTDIR)$(confdir)/rules.d/vim.rules
	rmdir --ignore-fail-on-non-empty $(DESTDIR)$(confdir)/rules.d
	rmdir --ignore-fail-on-non-empty $(DESTDIR)$(confdir)

uninstall: uninstall-scripts uninstall-man uninstall-lib uninstall-data \
	uninstall-conf

.PHONY: all clean distclean man deb dist install-scripts install-man install-lib \
	install-data install-conf install uninstall-scripts uninstall-man \
	uninstall-lib uninstall-data uninstall-conf uninstall
