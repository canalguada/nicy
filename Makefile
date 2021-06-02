SHELL		= /bin/bash
DESTDIR		?=
prefix		?= /usr/local
bindir		= $(prefix)/bin
confdir		= $(prefix)/etc/nicy
datadir		= $(prefix)/share/nicy

install-scripts:
	install -d $(DESTDIR)$(bindir)
	install -m755 nicy $(DESTDIR)$(bindir)/

install-data:
	install -d $(DESTDIR)$(datadir)/jq
	find jq/ -type f -iname "*.jq" -exec install -m755 {} $(DESTDIR)$(datadir)/jq/ \;
	install -m755 usage.json $(DESTDIR)$(datadir)/
	install -m755 procstat.awk $(DESTDIR)$(datadir)/

install-conf:
	install -d $(DESTDIR)$(confdir)/rules.d
	install -m644 environment $(DESTDIR)$(confdir)/
	install -m644 00-cgroups.cgroups $(DESTDIR)$(confdir)/
	install -m644 00-types.types $(DESTDIR)$(confdir)/
	install -m644 rules.d/vim.rules $(DESTDIR)$(confdir)/rules.d


install: install-scripts install-data install-conf

uninstall-scripts:
	rm -f $(DESTDIR)$(bindir)/nicy

uninstall-data:
	rm -rf $(DESTDIR)$(datadir)

uninstall-conf:
	rm -f $(DESTDIR)$(confdir)/environment
	rm -f $(DESTDIR)$(confdir)/00-cgroups.cgroups
	rm -f $(DESTDIR)$(confdir)/00-types.types
	rm -f $(DESTDIR)$(confdir)/rules.d/vim.rules
	rmdir --ignore-fail-on-non-empty $(DESTDIR)$(confdir)/rules.d
	rmdir --ignore-fail-on-non-empty $(DESTDIR)$(confdir)

uninstall: uninstall-scripts uninstall-data uninstall-conf

.PHONY: install-scripts install-data install-conf install uninstall-scripts uninstall-data uninstall-conf uninstall
