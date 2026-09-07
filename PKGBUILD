# Maintainer: luke <mail@lukeking12.net>
# Fork of the upstream 'curd' package (https://github.com/Wraient/curd) that
# builds from source, because the AnimeWorld provider for Italian audio and
# subtitles only exists in this fork and not in the upstream release binaries.
pkgname='curd-ita'
pkgver=2.0.7
pkgrel=1
pkgdesc="Watch anime in CLI with AniList Tracking, Discord RPC, Intro/Outro/Filler/Recap Skipping, and Italian subs and dubs via AnimeWorld"
arch=('x86_64' 'aarch64')
url="https://github.com/FalseEnvironment/curd-ita"
license=('GPL3')
depends=('mpv')
makedepends=('go' 'git')
optdepends=('rofi: selection menu'
            'ueberzugpp: image preview in rofi'
            'chromium: required by the Animepahe provider')
provides=('curd')
conflicts=('curd')
source=("git+https://github.com/FalseEnvironment/curd-ita.git#branch=main")
sha256sums=('SKIP')

pkgver() {
  cd "$srcdir/$pkgname"
  printf '%s.r%s.g%s' "$(tr -d '[:space:]' < VERSION.txt)" "$(git rev-list --count HEAD)" "$(git rev-parse --short HEAD)"
}

build() {
  cd "$srcdir/$pkgname"
  go build -mod=vendor -trimpath -ldflags '-s -w' -o curd ./cmd/curd
}

package() {
  cd "$srcdir/$pkgname"
  install -Dm755 curd "$pkgdir/usr/bin/curd"
  install -Dm644 LICENSE "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
}
