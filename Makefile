SHELL		= /bin/bash
DESTDIR		?=
prefix		?= /usr/local
bindir		= $(prefix)/bin
confdir		= $(prefix)/etc/nicy

install-scripts:
	install -d $(DESTDIR)$(bindir)
	install -m755 nicy $(DESTDIR)$(bindir)/
	install -m755 nicy-path-helper $(DESTDIR)$(bindir)/

install-conf:
	install -d $(DESTDIR)$(confdir)
	install -m644 environment $(DESTDIR)$(confdir)/
	install -m644 00-cgroups.cgroups $(DESTDIR)$(confdir)/
	install -m644 00-types.types $(DESTDIR)$(confdir)/

install: install-scripts install-conf

uninstall-scripts:
	rm -f $(DESTDIR)$(bindir)/nicy
	rm -f $(DESTDIR)$(bindir)/nicy-path-helper

uninstall-conf:
	rm -f $(DESTDIR)$(confdir)/environment
	rm -f $(DESTDIR)$(confdir)/00-cgroups.cgroups
	rm -f $(DESTDIR)$(confdir)/00-types.types
	rmdir --ignore-fail-on-non-empty $(DESTDIR)$(confdir)

uninstall: uninstall-scripts uninstall-conf

.PHONY: install-scripts install-conf install uninstall-scripts uninstall-conf uninstall
