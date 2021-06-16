pkgname=nicy-git
_pkgname=nicy
pkgver=0.1.4
pkgrel=1
pkgdesc="Set the execution environment of processes applying presets."
arch=(any)
url="https://github.com/canalguada/nicy"
license=("GPL3")
makedepends=("jq>=1.6")
depends=("jq>=1.6")
source=("${_pkgname}::git+https://github.com/canalguada/nicy.git")
sha256sums=("SKIP")

pkgver() {
  cd "$_pkgname"
  printf "%s.r%s.%s" \
    "$(git describe --tags)" \
    "$(git rev-list --count HEAD)" \
    "$(git rev-parse --short HEAD)" | \
      sed 's/-/.r/; s/-g/./; s/^v//'
}

package() {
  cd "$_pkgname"
  make DESTDIR="$pkgdir" install
}

# vim: set fdm=indent ai ts=2 sw=2 tw=79 et:
