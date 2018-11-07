#!/bin/bash

case "${OSTYPE}" in
  darwin*) ./install_prereqs_macos.sh;;
  linux*)
    DISTRO="$(lsb_release -i -s)"
    # If lsb_release is not installed on CentOS, DISTRO will be empty.
    if [[ -z "$DISTRO" && -f /etc/centos-release ]]; then
      DISTRO="CentOS"
    fi
    case "${DISTRO}" in
      Debian|Ubuntu)
        ./install_prereqs_debian.sh;;
      CentOS)
        ./install_prereqs_centos.sh;;
      *) echo "unsupported distro: ${DISTRO}" ;;
    esac;;
  *) echo "unsupported: ${OSTYPE}" ;;
esac
