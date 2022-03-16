#!/bin/bash

# echo "Creating big file to send over"
# dd if=/dev/urandom of=big_file bs=1G count=2 iflag=fullblock
# sha256sum big_file > big_file.sha256

((count = 100))                        
while [[ $count -ne 0 ]] ; do
    ping -c 1 $1                   
    rc=$?
    if [[ $rc -eq 0 ]] ; then
        ((count = 1))
    fi
    ((count = count - 1))
done

if [[ $rc -eq 0 ]] ; then
    echo "Alright"
else
    echo "Test failed"
    exit 1
fi

# host=$1

# if [ "$3" == "download" ]; then
#     set -e
#     echo "Downloading big file"
#     curl -v -L $host/big_file -O big_file
#     curl -v -L $host/big_file.sha256 -O big_file.sha256

#     echo "Verifying checksum"
#     sha256sum -c "big_file.sha256"

#     curl -X PUT http://localhost:8080/api/ledger/tests/vpn$2
#     sleep 30
#     curl -X PUT http://localhost:8080/api/ledger/tests/vpn$2

#     sleep 30

# else

#     set +e
#     ((count = 640))                        
#     while [[ $count -ne 0 ]] ; do
#         sleep 5
#         curl http://localhost:8080/api/ledger/tests/vpn$1 | grep "24"
#         rc=$?
#         if [[ $rc -eq 0 ]] ; then
#             ((count = 1))
#         fi
#         ((count = count - 1))
#     done

# fi