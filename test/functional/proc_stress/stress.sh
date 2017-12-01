#!/bin/sh

N=0
while [ $N -lt 256 ]
do
	./main $N
	N=$((N+1))
done

exit 0
