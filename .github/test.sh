#!/bin/bash

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
    sleep 20
    exit 0
else
    echo "Test failed"
    exit 1
fi