#!/bin/bash
./edgevpn api &

if [ $1 == "expose" ]; then
    ./edgevpn service-add --name "testservice" --remoteaddress "127.0.0.1:8080" &

    ((count = 100))                        
    while [[ $count -ne 0 ]] ; do
        sleep 2
        curl http://localhost:8080/api/ledger/tests/services | grep "doneservice"
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
    
else
    ./edgevpn service-connect --name "testservice" --srcaddress ":9090" &

    ((count = 100))                        
    while [[ $count -ne 0 ]] ; do
        sleep 2
        curl http://localhost:9090/ | grep "EdgeVPN"
        rc=$?
        if [[ $rc -eq 0 ]] ; then
            ((count = 1))
        fi
        ((count = count - 1))
    done

    if [[ $rc -eq 0 ]] ; then
        echo "Alright"
        curl -X PUT http://localhost:8080/api/ledger/tests/services/doneservice
        sleep 20
        exit 0
    else
        echo "Test failed"
        exit 1
    fi
fi


