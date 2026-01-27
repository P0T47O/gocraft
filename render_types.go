package main

type BlockGetter func(x, y, z int) byte
type LightGetter func(x, y, z int) byte
type MetaGetter func(x, y, z int) byte
