#!/bin/bash
./edgevpn api &

if [ $1 == "sender" ]; then
    echo "test" > $PWD/test

    ./edgevpn file-send --name "test" --path $PWD/test &

    ((count = 240))                        
    while [[ $count -ne 0 ]] ; do
        sleep 2
        curl http://localhost:8080/api/ledger/tests/test | grep "done"
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
    ./edgevpn file-receive --name "test" --path $PWD/test

    if [ ! -e $PWD/test ]; then
        echo "No file downloaded"
        exit 1
    fi

    curl -X PUT http://localhost:8080/api/ledger/tests/test/done

    t=$(cat $PWD/test)

    if [ $t != "test" ]; then
        echo "Failed test, returned $t"
        exit 1
    fi
fi


