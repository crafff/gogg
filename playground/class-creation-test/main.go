package main

import (
	"fmt"
)

type Summoner struct {
	ID   int
	Name string
	level int  // private field
}

func NewSummoner(id int, name string, level int) *Summoner {
	return &Summoner{
		ID: id,
		Name: name,
		level: level,
	}
}

func (s *Summoner) SetLevel(level int) {
	s.level = level
}

func (s Summoner) GetLevel() int {
	return s.level
}

func main() {
	summoner := NewSummoner(1, "AB", 10)
	fmt.Println("Name: ", summoner.Name)
	fmt.Println("Level: ", summoner.GetLevel())
	summoner.SetLevel(20)
	fmt.Println("Updated Level: ", summoner.level)

	summoner2 := Summoner{ID: 2, Name: "CD", level: 15}
	summoner2.SetLevel(25)
	fmt.Println("Name: ", summoner2.Name)
	fmt.Println("Level: ", summoner2.GetLevel())
}