#!/bin/bash

set -e 

SOURCE_ROOT=/source_root

mkdir -p /root/.ssh
chown root:root /root/.ssh
chmod 0700 /root/.ssh


# Copy general connections
if [ -f ${SOURCE_ROOT}/.netrc ]; then
    cp ${SOURCE_ROOT}/.netrc /root/.netrc
fi

# Copy the public key
if [ -f ${SOURCE_ROOT}/.ssh/id_rsa.pub ]; then
    cp ${SOURCE_ROOT}/.ssh/id_rsa.pub /root/.ssh/authorized_keys
fi

# Copy the known_hosts
if [ -f ${SOURCE_ROOT}/.ssh/known_hosts ]; then
    cp ${SOURCE_ROOT}/.ssh/known_hosts /root/.ssh/known_hosts
fi

ssh-keyscan -t rsa github.com >> /root/.ssh/known_hosts

/cork-server save-env

/usr/sbin/sshd -D
