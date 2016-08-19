#!/usr/bin/env bash
fail() {
    echo configuration failed
    exit 1
}
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
