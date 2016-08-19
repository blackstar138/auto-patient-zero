#!/usr/bin/env bash
fail() {
    echo configuration failed
    exit 1
}

if [ -z "$BASH_SOURCE" ]; then
    echo "This script *must* run under bash. Please rerun with '$ bash $0'"
    fail
fi

echo "Rebulding MIG Components for new modules"
which sudo 2>&1 1>/dev/null || (echo Install sudo and try again && exit 1)

echo -e "\n---- Shutting down existing Scheduler and API tmux sessions\n"
sudo tmux -S /tmp/tmux-$(id -u mig)/default kill-session -t mig || echo "OK - No running MIG session found"

if [[ -z $GOPATH ]]; then
    echo "GOPATH env variable is not set. setting it to '$HOME/go'"
    export GOPATH="$HOME/go"
fi

if [[ "$GOPATH/src/mig.ninja/mig" != "$(pwd)" ]]; then
    echo "Error: Setup of directories"
    echo "-----"
    echo "       - You should work in '$GOPATH/src/mig.ninja/mig'"
    echo "       - Current GOPATH is '$GOPATH'. current dir is '$(pwd)'."
    echo "       - Moving to '$GOPATH/src/mig.ninja/mig'."
    cd "$GOPATH/src/mig.ninja/mig"
fi

echo "--------------------------------------------------------------------"
echo -n "Would you like to build the MIG Scheduler Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then
    echo -e "\n---- Building MIG Scheduler\n"
    make mig-scheduler || fail
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
