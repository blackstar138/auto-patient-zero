#!/usr/bin/env bash
fail() {
    echo ****** Configuration failed *******
    exit 1
}

echo ---- RabbitMQ Server deployment script
which sudo 2>&1 1>/dev/null || (echo Install sudo and try again && exit 1)

echo "--------------------------------------------------------------------"
echo -e "\n---- Checking build environment\n"
echo "--------------------------------------------------------------------"

pkglist=""
installRabbitRPM=false
isRPM=false
distrib=$(head -1 /etc/issue|awk '{print $1}')
case $distrib in
    Debian|Ubuntu)
        PKG="apt-get"
        [ ! -e "/usr/include/readline/readline.h" ] && pkglist="$pkglist libreadline-dev"
        [ ! -d "/var/lib/rabbitmq" ] && pkglist="$pkglist rabbitmq-server"
    ;;
esac

echo -e "\n---- Checking the installed version of go\n"
# Make sure the correct version of go is installed. We need at least version
# 1.5.
#if [ ! $(which go) ]; then
#    echo "go doesn't seem to be installed, or is not in PATH; at least version 1.5 is required"
#    exit 1
#fi
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
echo -e "\nOK"

echo "--------------------------------------------------------------------"
echo -n "Would you like to configure the RabbitMQ Relay Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then

    echo -e "\n---- Enter RabbitMQ Password: "
    read mqpass
    if [[ mqpass == "" ]]; then
        echo "No password entered. Try again> "
        read mqpass
        if [[ mqpass == "" ]]; then
            echo "No password entered. Exiting..."
        fi
    fi

    echo -e "\n---- Configuring RabbitMQ\n"
    (ps faux|grep "/var/lib/rabbitmq"|grep -v grep) 2>&1 1> /dev/null
    if [ $? -gt 0 ]; then
        sudo service rabbitmq-server restart || fail
    fi

    mqpass=$(cat /dev/urandom | tr -dc _A-Z-a-z-0-9 | head -c${1:-32})
    echo -e "\nAttempt to delete existing admin user..."
    sudo rabbitmqctl delete_user admin
    echo -e "\nAttempt to create new admin user..."
    sudo rabbitmqctl add_user admin $mqpass || fail
    sudo rabbitmqctl set_user_tags admin administrator || fail

    echo -e "\nAttempt to delete existing vhost mig..."
    sudo rabbitmqctl delete_vhost mig
    echo -e "\nAttempt to create new vhost mig..."
    sudo rabbitmqctl add_vhost mig || fail
    sudo rabbitmqctl list_vhosts || fail

    echo -e "\nAttempt to delete existing user scheduler..."
    sudo rabbitmqctl delete_user scheduler
    echo -e "\nAttempt to add new user scheduler..."
    sudo rabbitmqctl add_user scheduler $mqpass || fail
    echo -e "\nAttempt to set permissions for user 'scheduler' on mig..."
    sudo rabbitmqctl set_permissions -p mig scheduler \
        '^(toagents|toschedulers|toworkers|mig\.agt\..*)$' \
        '^(toagents|toworkers|mig\.agt\.(heartbeats|results))$' \
        '^(toagents|toschedulers|toworkers|mig\.agt\.(heartbeats|results))$' || fail

    echo -e "\nAttempt to delete existing user agent..."
    sudo rabbitmqctl delete_user agent
    echo -e "\nAttempt to create new user agent..."
    sudo rabbitmqctl add_user agent $mqpass || fail
    echo -e "\nAttempt to set permissions for user 'agent' on mig..."
    sudo rabbitmqctl set_permissions -p mig agent \
        '^mig\.agt\..*$' \
        '^(toschedulers|mig\.agt\..*)$' \
        '^(toagents|mig\.agt\..*)$' || fail

    echo -e "\nAttempt to delete existing user worker..."
    sudo rabbitmqctl delete_user worker
    echo -e "\nAttempt to create new user 'worker'..."
    sudo rabbitmqctl add_user worker $mqpass || fail
    echo -e "\nAttempt to set permissions for user 'worker' on mig..."
    sudo rabbitmqctl set_permissions -p mig worker \
        '^migevent\..*$' \
        '^migevent(|\..*)$' \
        '^(toworkers|migevent\..*)$'

    echo "Attempting to restart RabbitMQ Service"
    sudo service rabbitmq-server restart || fail
fi

echo "--------------------------------------------------------------------"
echo -e "\n---- RabbitMQ Server Setup Completed! -----\n"
echo "--------------------------------------------------------------------"
