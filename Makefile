SHELL		= /bin/bash
DESTDIR		?=
package		= nicy
program		= nicy
git_branch	= master
version		= 0.2.0
revision	= 1
release_dir	= build
prefix		= /usr/local
bindir		= $(prefix)/bin
confdir		= $(prefix)/etc/$(program)

SUDO		= $$([ $$UID -ne 0 ] && echo "sudo")

.DEFAULT_GOAL := default

.PHONY: build
build:
	/usr/bin/go build -o $(program) -ldflags="-s" .

.PHONY: capabilities
capabilities:
	$(SUDO) /usr/sbin/setcap "cap_setpcap,cap_sys_nice,cap_sys_resource=p" ./$(program)

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
	cd man ; \
	sed -e 's#%prefix%#$(prefix)#g' -e 's#%version%#$(version)#g' \
		$(program).1.md  | \
	pandoc -s -f markdown -t man > $(program).1 ; gzip -9 -f $(program).1
	cd man ; \
	sed -e 's#%prefix%#$(prefix)#g' -e 's#%version%#$(version)#g' \
		$(program).5.md  | \
	pandoc -s -f markdown -t man > $(program).5 ; gzip -9 -f $(program).5


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
	install -d $(DESTDIR)$(prefix)/share/man/man5
	install -m644 man/$(program).5.gz $(DESTDIR)$(prefix)/share/man/man5/

.PHONY: install-conf
install-conf:
	install -m644 -D -T conf/config.yaml $(DESTDIR)$(confdir)/v$(version).yaml


.PHONY: install
install: install-bin install-man install-conf

.PHONY: uninstall-bin
uninstall-bin:
	rm -f $(DESTDIR)$(bindir)/$(program)

.PHONY: uninstall-man
uninstall-man:
	rm -f $(DESTDIR)$(prefix)/share/man/man1/$(program).1.gz
	rm -f $(DESTDIR)$(prefix)/share/man/man5/$(program).5.gz

.PHONY: uninstall-conf
uninstall-conf:
	rm -f $(DESTDIR)$(confdir)/v$(version).yaml
	rmdir --ignore-fail-on-non-empty $(DESTDIR)$(confdir)

.PHONY: uninstall
uninstall: uninstall-bin uninstall-man uninstall-conf

