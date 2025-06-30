package tml

import "etools/typedef"

type Restult struct {
	Success   bool
	Territory *typedef.Territory
}

type ResultInterface insterface {
	String() string
	Success() bool
	Territory() *typedef.Territory
}

type TerritoryResult struct {
	Success   bool
	Territory *typedef.Territory
}

func (r *TerritoryResult) String() string {
	
}

func Parse()
