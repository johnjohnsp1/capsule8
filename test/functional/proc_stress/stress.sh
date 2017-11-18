#!/bin/bash

for ((N=0; N<256; ++N))
do
    ./main $N &
done

wait

exit 0
