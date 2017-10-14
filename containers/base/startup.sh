#!/bin/bash

set -e 

mkdir -p /root/.ssh
chown root:root /root/.ssh
chmod 0700 /root/.ssh

# Copy general connections
if [ -f ${CORK_HOST_HOME_DIR}/.netrc ]; then
    cp ${CORK_HOST_HOME_DIR}/.netrc /root/.netrc
fi

# Copy the public key
if [ -f ${CORK_HOST_HOME_DIR}/.ssh/id_rsa.pub ]; then
    cp ${CORK_HOST_HOME_DIR}/.ssh/id_rsa.pub /root/.ssh/authorized_keys
fi

# Copy the known_hosts
if [ -f ${CORK_HOST_HOME_DIR}/.ssh/known_hosts ]; then
    cp ${CORK_HOST_HOME_DIR}/.ssh/known_hosts /root/.ssh/known_hosts
fi

ssh-keyscan -t rsa github.com >> /root/.ssh/known_hosts

/cork-server/cork-server save-env

/usr/sbin/sshd -D
