#!/bin/bash

# Allocate memory rapidly to trigger the OOM Killer
# This script creates a large array and keeps doubling its size

# Initial size of the array
size=100000

echo 'test 1' >output/example

sleep 60 # wait for first output checkpoint

echo 'test 2' >output/example

sleep 60 # wait for second output checkpoint

echo 'test 3' >output/example

# Infinite loop until OOM
while true; do
	echo "Allocating array of size: $size"

	# Create an array of the specified size
	array=$(seq 1 $size)

	# Double the size for the next iteration
	size=$((size * 2))

	# Small delay to slow down the loop slightly
	sleep 1
done
