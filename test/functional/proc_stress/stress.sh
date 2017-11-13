#!/bin/bash

for ((N=0; N<256; ++N))
do
    ./main $N
done

exit 0
