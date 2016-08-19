#!/usr/bin/env bash
fail() {
    echo configuration failed
    exit 1
}

echo ---- Begin Mozilla MIG Server deployment script
which sudo 2>&1 1>/dev/null || (echo Install sudo and try again && exit 1)

MAKEGOPATH=false
if [ "$1" == "makegopath" ]; then
    MAKEGOPATH=true
fi
echo "--------------------------------------------------------------------"
echo -e "\n---- Checking build environment\n"
if [[ -z $GOPATH && $MAKEGOPATH == "true" ]]; then
    echo "GOPATH env variable is not set. setting it to '$HOME/go'"
    export GOPATH="$HOME/go"
fi
# if [[ -z $GOPATH && $MAKEGOPATH == "false" ]]; then
#     echo "GOPATH env variable is not set. either set it, or ask this script to create it using: $ $0 makegopath"
#     fail
# fi

# if [[ ! -d "$GOPATH/src/mig.ninja/mig" ]]; then
#     echo 'MIG sources not found. Attempting to download with "go get mig.ninja/mig"'
#     which git  2>&1 1>/dev/null || echo "git library missing! Download? y/n> "
#     read yesno
#     if [ $yesno == "no" ]; then
#         fail
#     fi
#     sudo apt-get install git || fail
#     go get mig.ninja/mig || fail
# fi

if [[ "$GOPATH/src/mig.ninja/mig" != "$(pwd)" ]]; then
    echo "Error: Setup of directories"
    echo "-----"
    echo "       - You should work in '$GOPATH/src/mig.ninja/mig'"
    echo "       - Current GOPATH is '$GOPATH'. current dir is '$(pwd)'."
    echo "       - Moving to '$GOPATH/src/mig.ninja/mig'."
    cd "$GOPATH/src/mig.ninja/mig"
fi
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
# if [ ! $(which go) ]; then
#     echo "go doesn't seem to be installed, or is not in PATH; at least version 1.5 is required"
#     exit 1
# fi
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

echo "--------------------------------------------------------------------"
echo -n "Would you like to build the MIG Scheduler Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then
    echo -e "\n---- Building MIG Scheduler\n"
    make mig-scheduler || fail
    echo -e "\n testing 'mig' user"
    id mig || echo "Creating user 'mig'...." && sudo useradd -r mig || fail
    echo OK
    sudo cp bin/linux/amd64/mig-scheduler /usr/local/bin/ || fail
    sudo chown mig /usr/local/bin/mig-scheduler || fail
    sudo chmod 550 /usr/local/bin/mig-scheduler || fail
fi

echo OK

echo "--------------------------------------------------------------------"
echo -n "Would you like to build the MIG API Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then
    echo -e "\n---- Building MIG API\n"
    make mig-api || fail
    sudo cp bin/linux/amd64/mig-api /usr/local/bin/ || fail
    sudo chown mig /usr/local/bin/mig-api || fail
    sudo chmod 550 /usr/local/bin/mig-api || fail
fi

echo OK

echo "--------------------------------------------------------------------"
echo -n "Would you like to build the MIG Worker Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then
    echo -e "\n---- Building MIG Worker\n"
    make worker-agent-verif || fail
    sudo cp bin/linux/amd64/mig-worker-agent-verif /usr/local/bin/ || fail
    sudo chown mig /usr/local/bin/mig-worker-agent-verif || fail
    sudo chmod 550 /usr/local/bin/mig-worker-agent-verif || fail
fi

echo OK

echo "--------------------------------------------------------------------"
echo -n "Would you like to build the MIG Clients Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then
    echo -e "\n---- Building MIG Clients\n"
    make mig-console || fail
    sudo cp bin/linux/amd64/mig-console /usr/local/bin/ || fail
    sudo chown mig /usr/local/bin/mig-console || fail
    sudo chmod 555 /usr/local/bin/mig-console || fail

    make mig-cmd || fail
    sudo cp bin/linux/amd64/mig /usr/local/bin/ || fail
    sudo chown mig /usr/local/bin/mig || fail
    sudo chmod 555 /usr/local/bin/mig || fail
fi
echo OK

echo "--------------------------------------------------------------------"
echo -n "Would you like to build the MIG Database Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then
    echo -e "\n---- Building Database\n"
    cd database/
    echo -e "\n---- Enter Postgres DB Password: "
    read dbpass
    #dbpass=$(cat /dev/urandom | tr -dc _A-Z-a-z-0-9 | head -c${1:-32})
    # dbpass=$(cat /dev/urandom | tr -dc _A-Z-a-z-0-9 | head -c32)
    #dbpass = $(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w ${1:-32} | head -n 1)
    echo "dbpass = $dbpass" >> $GOPATH/MIG_setup.txt
    #sudo su - postgres -c "psql -c 'drop database mig'"
    #sudo su - postgres -c "psql -c 'drop role migadmin'"
    #sudo su - postgres -c "psql -c 'drop role migapi'"
    #sudo su - postgres -c "psql -c 'drop role migscheduler'"
    #sudo su - postgres -c "psql -c 'drop role migreadonly'"
    bash createlocaldb.sh $dbpass || fail
    cd ..
fi
echo OK

echo "--------------------------------------------------------------------"
echo -n "Would you like to build the MIG System & Folders Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then
    echo -e "\n---- Creating system user and folders\n"
    sudo mkdir -p /var/cache/mig/{action/new,action/done,action/inflight,action/invalid,command/done,command/inflight,command/ready,command/returned} || fail
    hostname > /tmp/agents_whitelist.txt
    hostname --fqdn >> /tmp/agents_whitelist.txt
    echo localhost >> /tmp/agents_whitelist.txt
    sudo mv /tmp/agents_whitelist.txt /var/cache/mig/
    sudo chown mig /var/cache/mig -R || fail
    [ ! -d /etc/mig ] && sudo mkdir /etc/mig
    sudo chown mig /etc/mig || fail
fi

echo "\nOK"

echo "--------------------------------------------------------------------"
echo -n "Would you like to generate the RabbitMQ Password Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then
    echo -e "\n---- Generate RabbitMQ Pass\n"
    mqpass=$(cat /dev/urandom | tr -dc _A-Z-a-z-0-9 | head -c32)
    echo "mqpass = $mqpass" >> $GOPATH/MIG_setup.txt
fi

if [[ -z $mqpass ]]; then
    echo "-- Failed to generate password"
    fail
fi

echo OK

echo "--------------------------------------------------------------------"
echo -n "Would you like to create the Scheduler Config Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then
    echo -e "\n---- Creating Scheduler configuration\n"
    cp conf/scheduler.cfg.inc /tmp/scheduler.cfg
    sed -i "s|whitelist = \"/var/cache/mig/agents_whitelist.txt\"|whitelist = \"\"|" /tmp/scheduler.cfg || fail
    sed -i "s/freq = \"87s\"/freq = \"3s\"/" /tmp/scheduler.cfg || fail
    sed -i "s/password = \"123456\"/password = \"$dbpass\"/" /tmp/scheduler.cfg || fail
    sed -i "s/user  = \"guest\"/user = \"scheduler\"/" /tmp/scheduler.cfg || fail
    sed -i "s/pass  = \"guest\"/pass = \"$mqpass\"/" /tmp/scheduler.cfg || fail
    sudo mv /tmp/scheduler.cfg /etc/mig/scheduler.cfg || fail
    sudo chown mig /etc/mig/scheduler.cfg || fail
    sudo chmod 750 /etc/mig/scheduler.cfg || fail
    echo OK
fi

echo "--------------------------------------------------------------------"
echo -n "Would you like to create the API Config Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then
    echo -e "\n---- Creating API configuration\n"
    cp conf/api.cfg.inc /tmp/api.cfg
    sed -i "s/password = \"123456\"/password = \"$dbpass\"/" /tmp/api.cfg || fail
    sudo mv /tmp/api.cfg /etc/mig/api.cfg || fail
    sudo chown mig /etc/mig/api.cfg || fail
    sudo chmod 750 /etc/mig/api.cfg || fail
    echo OK
fi

echo "--------------------------------------------------------------------"
echo -n "Would you like to create the Worker Config Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then
    echo -e "\n---- Creating Worker configuration\n"
    cp conf/agent-verif-worker.cfg.inc /tmp/agent-verif-worker.cfg
    sed -i "s/pass = \"secretpassphrase\"/pass = \"$mqpass\"/" /tmp/agent-verif-worker.cfg || fail
    sudo mv /tmp/agent-verif-worker.cfg /etc/mig/agent-verif-worker.cfg || fail
    sudo chown mig /etc/mig/agent-verif-worker.cfg || fail
    sudo chmod 750 /etc/mig/agent-verif-worker.cfg || fail
    echo OK
fi

echo "--------------------------------------------------------------------"
echo -n "Would you like to start the Scheduler & API Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then
    echo -e "\n---- Starting Scheduler and API in TMUX under mig user\n"
    sudo su mig -c "/usr/bin/tmux new-session -s 'mig' -d"
    sudo su mig -c "/usr/bin/tmux new-window -t 'mig' -n '0' '/usr/local/bin/mig-scheduler'"
    sudo su mig -c "/usr/bin/tmux new-window -t 'mig' -n '0' '/usr/local/bin/mig-api'"
    sudo su mig -c "/usr/bin/tmux new-window -t 'mig' -n '0' '/usr/local/bin/mig_agent_verif_worker'"
    echo OK
fi

# Unset proxy related environment variables from this point on, since we want to ensure we are
# directly accessing MIG resources locally.
if [ ! -z "$http_proxy" ]; then
    unset http_proxy
fi
if [ ! -z "$https_proxy" ]; then
    unset https_proxy
fi

echo "--------------------------------------------------------------------"
echo -n "Would you like to test the API status Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then
    echo -e "\n---- Testing API status\n"
    sleep 2
    ret=$(curl -s http://localhost:12345/api/v1/heartbeat | grep "gatorz say hi")
    [ "$?" -gt 0 ] && fail
    echo OK - API is running
fi

echo "--------------------------------------------------------------------"
echo -n "Would you like to create the Client config & setup new investigator Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then

    echo -e "\n---- Creating GnuPG key pair for new investigator in ~/.mig/\n"
    [ ! -d ~/.mig ] && mkdir ~/.mig
    gpg --batch --no-default-keyring --keyring ~/.mig/pubring.gpg --secret-keyring ~/.mig/secring.gpg --gen-key << EOF
    Key-Type: 1
    Key-Length: 1024
    Subkey-Type: 1
    Subkey-Length: 1024
    Name-Real: $(whoami) Investigator
    Name-Email: $(whoami)@$(hostname)
    Expire-Date: 12m

EOF

    echo -e "\n---- Creating client configuration in ~/.migrc\n"
    keyid=$(gpg --no-default-keyring --keyring ~/.mig/pubring.gpg \
        --secret-keyring ~/.mig/secring.gpg --fingerprint \
        --with-colons $(whoami)@$(hostname) | grep '^fpr' | cut -f 10 -d ':')
    cat > ~/.migrc << EOF
    [api]
        url = "http://localhost:12345/api/v1/"
        skipverifycert = on
    [gpg]
        home = "$HOME/.mig/"
        keyid = "$keyid"

EOF

    echo -e "\n---- Creating investigator $(whoami) in database\n"
    gpg --no-default-keyring --keyring ~/.mig/pubring.gpg \
        --secret-keyring ~/.mig/secring.gpg \
        --export -a $(whoami)@$(hostname) \
        > ~/.mig/$(whoami)-pubkey.asc || fail
    echo -e "create investigator\n$(whoami)\nyes\n$HOME/.mig/$(whoami)-pubkey.asc\ny\n" | \
        /usr/local/bin/mig-console -q || fail

fi

echo "End of Client config / new investigator / new GPG key pair --> OK"

echo "--------------------------------------------------------------------"
echo -e "\n---- MIG Server Setup Completed! -----\n"
echo "--------------------------------------------------------------------"
cd $HOME
cat postintsall.txt
