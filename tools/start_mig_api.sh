#!/usr/bin/env bash
fail() {
    echo configuration failed
    exit 1
}

if [ -z "$BASH_SOURCE" ]; then
    echo "This script *must* run under bash. Please rerun with '$ bash $0'"
    fail
fi

echo Standalone MIG demo deployment script
which sudo 2>&1 1>/dev/null || (echo Install sudo and try again && exit 1)

echo "--------------------------------------------------------------------"
echo -n "Starting Scheduler and API in TMUX under mig user "
echo "--------------------------------------------------------------------"
sudo su mig -c "/usr/bin/tmux new-session -s 'mig' -d" || fail
sudo su mig -c "/usr/bin/tmux new-window -t 'mig' -n '0' '/usr/local/bin/mig-scheduler'" || fail
sudo su mig -c "/usr/bin/tmux new-window -t 'mig' -n '0' '/usr/local/bin/mig-api'" || fail
sudo su mig -c "/usr/bin/tmux new-window -t 'mig' -n '0' '/usr/local/bin/mig_agent_verif_worker'" || fail
echo OK

# Unset proxy related environment variables from this point on, since we want to ensure we are
# directly accessing MIG resources locally.
if [ ! -z "$http_proxy" ]; then
    unset http_proxy
fi
if [ ! -z "$https_proxy" ]; then
    unset https_proxy
fi

echo "--------------------------------------------------------------------"
echo -n "Testing API status "
echo "--------------------------------------------------------------------"

sleep 2
ret=$(curl -s http://localhost:12345/api/v1/heartbeat | grep "gatorz say hi")
[ "$?" -gt 0 ] && fail
echo OK - API is running
