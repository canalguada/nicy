#!/bin/bash
# vim: set ft=sh fdm=indent ai ts=2 sw=2 tw=79 noet:

# Replace Makefile shell blobs
#
# deb: dist
#   cd $(release_dir) && tar -xf $(package)-$(version).tar.gz && \
#   cd $(package)-$(version) && \
#   debmake -b"$(package):bin" -u"$(version)" -r"$(revision)" && \
#   debuild
#
# deb:
#   # Prepare with dh-make-golang make -force_prerelease  -git_revision $(git_branch) -type p github.com/canalguada/nicy
#   # To build the package, commit the packaging and use gbp buildpackage:
#   # git add debian && git commit -S -m 'Initial packaging'
#   gbp buildpackage --git-pbuilder
#
# deb:
#   debver=$$(git describe --long --tags | sed 's/v\(.*\)-\(.*\)-\(.*\)/\1\+git'$$(date +"%Y%m%d")'\.\3/g')-$(revision) ; \
#   arch=$$(dpkg --print-architecture) ; \
#   debname="$(package)-$${debver}_$${arch}" ; \
#   rm -f "$(release_dir)/$${debname}.deb" ; \
#   $(SUDO) rm -rf "$(release_dir)/$${debname}" ; \
#   install -d "$(release_dir)/$${debname}/DEBIAN" ; \
#   make DESTDIR="$(release_dir)/$${debname}" prefix="/usr" confdir="/etc/$(program)" install ; \
#   sed \
#   -e 's/\(Version:\) VERSION/\1 '$${debver}'/g'  \
#   -e 's/\(Architecture:\) ARCH/\1 '$${arch}'/g'  \
#   ./DEBIAN/control >"$(release_dir)/$${debname}/DEBIAN/control" ; \
#   cd $(release_dir) ; \
#   $(SUDO) chown -R root:root "$${debname}" && dpkg-deb --build "$${debname}"
#

set -o pipefail
set -o errtrace
set -o nounset
set -o errexit

# Utils {{{
# Error codes
SUCCESS=0
FAILURE=1
EPARSE=2
EINVAL=6

package=$(basename `dirname $(realpath $0)`)
program=$package

error_exit() {
  echo "$package: error: ${2:-'unknown error'}" >&2
  exit "${1:-$FAILURE}"
}

usage() {
  cat<<_EOF_
  Build nicy package for Debian-based distributions

  usage: ./$(basename $0)
_EOF_
}

if [ $# -gt 0 ]; then
  usage
  error_exit $EPARSE "too many arguments"
fi

# Check for required commands
LANG=C command -V sed
LANG=C command -V git
LANG=C command -V make

# Read environment from Makefile, if missing
set +o nounset
for item in "version" "revision" "release_dir"; do
  if [ -z "${!item}" ]; then
    eval $item=$(grep '^'$item'.*=' Makefile | sed 's/'$item'.*=\s//')
  fi
done
set -o nounset

# Debian-based distributions
if command -v lsb_release &>/dev/null &&
	[[ "$(lsb_release -is | tr "[:upper:]" "[:lower:]")" =~ debian|ubuntu ]]; then
  # More requred commands
  LANG=C command -V dpkg
  LANG=C command -V dpkg-deb
  # Build package version and name
  debver=$( \
    git describe --long --tags | \
    sed 's/v\(.*\)-\(.*\)-\(.*\)/\1\+git'$(date +"%Y%m%d")'\.\3/g'
  )
  debver="${debver}-${revision}"
  arch=$(dpkg --print-architecture)
  debname="${package}_${debver}_${arch}"
  # Prepare release directory
  rm -f "${release_dir}/${debname}.deb"
  rm -rf "${release_dir}/${debname}"
  install -d "${release_dir}/${debname}/DEBIAN"
  # Copy release
  make \
    DESTDIR="${release_dir}/${debname}" \
    prefix="/usr" confdir="/etc/${program}" \
    install
  # Update version and revision
  sed \
    -e 's/\(Version:\) VERSION/\1 '${debver}'/g'  \
    -e 's/\(Architecture:\) ARCH/\1 '${arch}'/g'  \
    ./DEBIAN/control >"${release_dir}/${debname}/DEBIAN/control"
	cp ./DEBIAN/postinst "${release_dir}/${debname}/DEBIAN/"
	chmod -f 0775 "${release_dir}/${debname}/DEBIAN/postinst"
  # Finally, build debian package
  cd ${release_dir}
	dpkg-deb --build --root-owner-group "${debname}"
fi
