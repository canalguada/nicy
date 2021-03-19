DESTDIR :=
prefix  := /usr/local
bindir  := ${prefix}/bin
confdir := ${prefix}/etc/nicy
systemdir := /etc/systemd/system
# TODO: use nproc when creating slice units
QUOTAS	= 8 16 25 33 50 66 75 80 90

.PHONY: install
install:
	install -d ${DESTDIR}${bindir}
	install -m755 nicy ${DESTDIR}${bindir}/
	install -m755 nicy-path-helper ${DESTDIR}${bindir}/
	install -d ${DESTDIR}${systemdir}
	$(foreach quota,$(QUOTAS),echo "[Slice]\nCPUQuota=$(quota)%\n" > ${DESTDIR}${systemdir}/cpu$(quota).slice;)
	install -d ${DESTDIR}${confdir}
	install -m644 environment ${DESTDIR}${confdir}/
	install -m644 00-cgroups.cgroups ${DESTDIR}${confdir}/
	install -m644 00-types.types ${DESTDIR}${confdir}/

.PHONY: uninstall
uninstall:
	rm -f ${DESTDIR}${bindir}/nicy
	rm -f ${DESTDIR}${bindir}/nicy-path-helper
	$(foreach quota,$(QUOTAS),rm -f ${DESTDIR}${systemdir}/cpu$(quota).slice;)
	rm -f ${DESTDIR}${confdir}/environment
	rm -f ${DESTDIR}${confdir}/00-cgroups.cgroups
	rm -f ${DESTDIR}${confdir}/00-types.types
