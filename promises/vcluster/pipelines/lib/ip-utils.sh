#!/usr/bin/env bash

is_valid_ipv4() {
  local ip=$1
  IFS=. read -r o1 o2 o3 o4 <<<"${ip}"
  for octet in "${o1}" "${o2}" "${o3}" "${o4}"; do
    if ! [[ "${octet}" =~ ^[0-9]+$ ]] || [ "${octet}" -gt 255 ]; then
      return 1
    fi
  done
  return 0
}

ip_to_int() {
  IFS=. read -r o1 o2 o3 o4 <<<"$1"
  echo $(( (o1 << 24) + (o2 << 16) + (o3 << 8) + o4 ))
}

int_to_ip() {
  local ip_int=$1
  echo "$(( (ip_int >> 24) & 255 )).$(( (ip_int >> 16) & 255 )).$(( (ip_int >> 8) & 255 )).$(( ip_int & 255 ))"
}

ip_in_cidr() {
  local ip=$1
  local cidr=$2
  local cidr_ip prefix mask ip_int cidr_int

  IFS=/ read -r cidr_ip prefix <<<"${cidr}"
  if [ -z "${cidr_ip}" ] || [ -z "${prefix}" ] || ! [[ "${prefix}" =~ ^[0-9]+$ ]] || [ "${prefix}" -gt 32 ]; then
    return 1
  fi
  if ! is_valid_ipv4 "${ip}" || ! is_valid_ipv4 "${cidr_ip}"; then
    return 1
  fi

  ip_int=$(ip_to_int "${ip}")
  cidr_int=$(ip_to_int "${cidr_ip}")
  mask=$(( 0xFFFFFFFF << (32 - prefix) & 0xFFFFFFFF ))
  if [ $(( ip_int & mask )) -ne $(( cidr_int & mask )) ]; then
    return 1
  fi

  return 0
}

default_vip_from_cidr() {
  local cidr=$1
  local offset=$2
  local cidr_ip prefix mask network_int host_count vip_int

  IFS=/ read -r cidr_ip prefix <<<"${cidr}"
  if [ -z "${cidr_ip}" ] || [ -z "${prefix}" ]; then
    return 1
  fi
  if ! is_valid_ipv4 "${cidr_ip}"; then
    return 1
  fi
  mask=$(( 0xFFFFFFFF << (32 - prefix) & 0xFFFFFFFF ))
  network_int=$(( $(ip_to_int "${cidr_ip}") & mask ))
  host_count=$(( 1 << (32 - prefix) ))
  if [ "${offset}" -ge "${host_count}" ]; then
    return 1
  fi
  vip_int=$(( network_int + offset ))
  int_to_ip "${vip_int}"
}
