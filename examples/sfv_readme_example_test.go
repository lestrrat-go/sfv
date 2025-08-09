package examples_test

import (
	"fmt"
	"log"

	"github.com/lestrrat-go/sfv"
)

func Example() {
	// Parse various SFV types

	// Parse a simple string item
	item, err := sfv.ParseItem([]byte(`"hello world"`))
	if err != nil {
		log.Fatal(err)
	}
	itemSerialized, _ := sfv.Marshal(item)
	fmt.Printf("Parsed string item: %s\n", string(itemSerialized))

	// Parse a list with mixed types
	list, err := sfv.Parse([]byte(`"text", 42, ?1, @1659578233`))
	if err != nil {
		log.Fatal(err)
	}
	listSerialized, _ := sfv.Marshal(list)
	fmt.Printf("Parsed list: %s\n", string(listSerialized))

	// Parse a dictionary
	dict, err := sfv.ParseDictionary([]byte(`key1="value1", key2=42, flag`))
	if err != nil {
		log.Fatal(err)
	}
	dictSerialized, _ := sfv.Marshal(dict)
	fmt.Printf("Parsed dictionary: %s\n", string(dictSerialized))

	// Create and serialize SFV values programmatically

	// Create a dictionary with various data types
	newDict := sfv.NewDictionary()
	newDict.Set("name", sfv.String("John Doe"))
	newDict.Set("age", sfv.Integer(30))
	newDict.Set("active", sfv.Boolean(true))
	newDict.Set("score", sfv.Decimal(98.5))
	newDict.Set("data", sfv.ByteSequence([]byte("hello")))

	// Serialize the dictionary
	serialized, err := sfv.Marshal(newDict)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Serialized dictionary: %s\n", string(serialized))

	// Work with parameters on items
	itemWithParams := sfv.String("cached-resource")
	itemWithParams.Parameter("max-age", 3600)
	itemWithParams.Parameter("public", true)

	paramSerialized, err := sfv.Marshal(itemWithParams)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Item with parameters: %s\n", string(paramSerialized))

	// Create a list with inner lists
	mainList := &sfv.List{}

	// Add simple items
	mainList.Add(sfv.String("item1"))
	mainList.Add(sfv.Integer(42))

	// Add an inner list with parameters
	innerList := &sfv.InnerList{}
	innerList.Add(sfv.String("inner1"))
	innerList.Add(sfv.String("inner2"))
	// Inner lists can have parameters too

	mainList.Add(innerList)

	complexSerialized, err := sfv.Marshal(mainList)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Complex list: %s\n", string(complexSerialized))

	// OUTPUT:
	// Parsed string item: "hello world"
	// Parsed list: "text", 42, ?1, @1659578233
	// Parsed dictionary: key1="value1", key2=42, flag
	// Serialized dictionary: name="John Doe", age=30, active, score=98.5, data=:aGVsbG8=:
	// Item with parameters: "cached-resource"; max-age=3600; public
	// Complex list: "item1", 42, ("inner1" "inner2")
}