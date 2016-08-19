#!/usr/bin/env bash
fail() {
    echo configuration failed
    exit 1
}

echo ---- Begin Mozilla MIG Server deployment script
echo "--------------------------------------------------------------------"
echo "---- Preparing packages dependencies"
echo "--------------------------------------------------------------------"
pkglist=""
installRabbitRPM=false
isRPM=false
distrib=$(head -1 /etc/issue|awk '{print $1}')
case $distrib in
    Debian|Ubuntu)
        PKG="apt-get"
        [ ! -e "/usr/include/readline/readline.h" ] && pkglist="$pkglist libreadline-dev"
        ls /usr/lib/postgresql/*/bin/postgres 2>&1 1>/dev/null || pkglist="$pkglist postgresql"
    ;;
esac

echo -e "\n---- Checking the installed version of go\n"
# Make sure the correct version of go is installed. We need at least version
# 1.5.
if [ ! $(which go) ]; then
    echo "go doesn't seem to be installed, or is not in PATH; at least version 1.5 is required"
    exit 1
fi
go_version=$(go version)
echo $go_version | grep -E -q --regexp="go1\.[0-4]" && echo -e "installed version of go is ${go_version}\nwe need at least version 1.5" && fail

which go   2>&1 1>/dev/null || pkglist="$pkglist golang"
which git  2>&1 1>/dev/null || pkglist="$pkglist git"
which hg   2>&1 1>/dev/null || pkglist="$pkglist mercurial"
which make 2>&1 1>/dev/null || pkglist="$pkglist make"
which gcc  2>&1 1>/dev/null || pkglist="$pkglist gcc"
which tmux 2>&1 1>/dev/null || pkglist="$pkglist tmux"
which curl 2>&1 1>/dev/null || pkglist="$pkglist curl"
which rngd 2>&1 1>/dev/null || pkglist="$pkglist rng-tools"

if [ "$pkglist" != "" ]; then
    echo "--------------------------------------------------------------------"
    echo "WARNING: Missing packages: $pkglist"
    echo "         -> Would you like to install the missing packages? (need sudo) y/n> "
    read yesno
    echo "--------------------------------------------------------------------"
    if [ $yesno = "y" ]; then
        sudo $PKG install $pkglist || fail
    fi
fi
echo "Packages installed!"

echo "--------------------------------------------------------------------"
echo -n "Would you like to continue with installation? y/n> "
read yesno
if [ $yesno = "n" ]; then
    echo -e "\n---- Exiting installation..."
    echo "--------------------------------------------------------------------"
    fail
fi
echo OK