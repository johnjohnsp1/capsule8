#!/bin/bash

for ((N=0; N<256; ++N))
do
    ./test-proc $N &
done
