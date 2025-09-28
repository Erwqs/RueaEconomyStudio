package tml

import "RueaES/typedef"

type Restult struct {
	Success   bool
	Territory *typedef.Territory
}

type ResultInterface interface {
	String() string
	Success() bool
	Territory() *typedef.Territory
}

type TerritoryResult struct {
	Success   bool
	Territory *typedef.Territory
}

func Parse()
