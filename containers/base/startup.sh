#!/bin/bash

set -eu

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
else
    if [ -f ${CORK_HOST_HOME_DIR}/.ssh/id_rsa ]; then
        ssh-keygen -y -f ${CORK_HOST_HOME_DIR}/.ssh/id_rsa > ${CORK_HOST_HOME_DIR}/.ssh/id_rsa.pub
        cp ${CORK_HOST_HOME_DIR}/.ssh/id_rsa.pub /root/.ssh/authorized_keys
    else
        echo "Private key not found. Keys must be in ~/.ssh/id_rsa at this time"
        exit 1
    fi
fi

# Copy the known_hosts
if [ -f ${CORK_HOST_HOME_DIR}/.ssh/known_hosts ]; then
    cp ${CORK_HOST_HOME_DIR}/.ssh/known_hosts /root/.ssh/known_hosts
fi

mkdir -p /root/.docker

ssh-keyscan -t rsa github.com >> /root/.ssh/known_hosts

/cork-server/cork-server save-env

/usr/sbin/sshd -D
