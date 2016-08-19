#!/usr/bin/env bash
fail() {
    echo configuration failed
    exit 1
}

echo "--------------------------------------------------------------------"
echo -n "Would you like to build the MIG Database Now? (need sudo) y/n> "
read yesno
echo "--------------------------------------------------------------------"
if [ $yesno = "y" ]; then


    if [[ "$GOPATH/src/mig.ninja/mig" != "$(pwd)" ]]; then
	echo "Error: Setup of directories"
	echo "-----"
	echo "       - You should work in '$GOPATH/src/mig.ninja/mig'"
	echo "       - Current GOPATH is '$GOPATH'. current dir is '$(pwd)'."
	echo "       - Moving to '$GOPATH/src/mig.ninja/mig'."
	cd "$GOPATH/src/mig.ninja/mig"
    fi
    echo -e "\n---- Building Database\n"

    cd database/

    dbpass="NK8z8Y4XP2Pfkc1-VKRZ83ZwVjB2CY8W"

    sudo su - postgres -c "psql -c 'drop database mig'"
    sudo su - postgres -c "psql -c 'drop role migadmin'"
    sudo su - postgres -c "psql -c 'drop role migapi'"
    sudo su - postgres -c "psql -c 'drop role migscheduler'"
    sudo su - postgres -c "psql -c 'drop role migreadonly'"
    bash createlocaldb.sh $dbpass || fail

    cd ..
fi
echo "MIG DB Rebuilt"
