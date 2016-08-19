#!/usr/bin/env bash
fail() {
    echo ****** Configuration failed *******
    exit 1
}

echo ---- RabbitMQ Server deployment script
which sudo 2>&1 1>/dev/null || (echo Install sudo and try again && exit 1)

echo "--------------------------------------------------------------------"
echo "---- Rebuild RabbitMQ Server Configuration"
echo "--------------------------------------------------------------------"

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

#mqpass=$(cat /dev/urandom | tr -dc _A-Z-a-z-0-9 | head -c${1:-32})
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
sudo service rabbitmq-server stop || fail
sudo rabbitmq-server || fail

echo "--------------------------------------------------------------------"
echo "---- RabbitMQ Configuration Completed! -----"
echo "--------------------------------------------------------------------"
