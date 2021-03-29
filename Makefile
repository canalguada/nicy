SHELL		= /bin/bash
DESTDIR		?=
prefix		?= /usr/local
bindir		= $(prefix)/bin
confdir		= $(prefix)/etc/nicy
libdir		= $(DESTDIR)$(prefix)/lib/systemd
ncpu		!= nproc --all
cpu_filter	= .[]|select(has("CPUQuota"))
slice_content	= [Slice]\\nCPUQuota=\((.CPUQuota|tonumber) * $(ncpu))%

install-scripts:
	install -d $(DESTDIR)$(bindir)
	install -m755 nicy $(DESTDIR)$(bindir)/
	install -m755 nicy-path-helper $(DESTDIR)$(bindir)/

install-conf:
	install -d $(DESTDIR)$(confdir)
	install -m644 environment $(DESTDIR)$(confdir)/
	install -m644 00-cgroups.cgroups $(DESTDIR)$(confdir)/
	install -m644 00-types.types $(DESTDIR)$(confdir)/

install-cgroups:
	for sd in "user" "system"; do \
		install -d $(libdir)/$$sd ; \
		jq_filter='$(cpu_filter)|' ; \
		jq_filter+='"echo -e \"$(slice_content)\" ' ; \
		jq_filter+='>$(libdir)/'"$$sd"'/\(.cgroup).slice ;"' ; \
		eval $$(grep -E -v "^[ ]*#|^$$" 00-cgroups.cgroups | jq -sMcr "$${jq_filter}") ; \
	done

install: install-scripts install-conf install-cgroups

uninstall-scripts:
	rm -f $(DESTDIR)$(bindir)/nicy
	rm -f $(DESTDIR)$(bindir)/nicy-path-helper

uninstall-conf:
	rm -f $(DESTDIR)$(confdir)/environment
	rm -f $(DESTDIR)$(confdir)/00-cgroups.cgroups
	rm -f $(DESTDIR)$(confdir)/00-types.types
	rmdir --ignore-fail-on-non-empty $(DESTDIR)$(confdir)

uninstall-cgroups:
	for sd in "user" "system"; do \
		jq_filter='$(cpu_filter)|' ; \
		jq_filter+='"rm -f $(libdir)/'"$$sd"'/\(.cgroup).slice ;"' ; \
		eval $$(grep -E -v "^[ ]*#|^$$" 00-cgroups.cgroups | jq -sMcr "$${jq_filter}") ; \
	done

uninstall: uninstall-scripts uninstall-conf uninstall-cgroups

.PHONY: install-scripts install-conf install-cgroups install
