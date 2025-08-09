package sfv

import "fmt"

// BareItemBuilder is a generic builder for creating BareItem instances with type safety.
// It provides a fluent interface for setting values and handling errors during construction.
type BareItemBuilder[B BareItem, T any] struct {
	value B
	// This is a horrible hack to allow us to
	setter func(T) error
	err    error
}

// ItemBuilder is a builder for creating Item instances with parameters.
// It wraps a BareItem and allows adding parameters in a fluent manner.
type ItemBuilder struct {
	value Item
	err   error
}

// ToItem upgrades the BareItemBuilder to an ItemBuilder, allowing the addition
// of parameters. Returns an ItemBuilder that can be used to add parameters
// and build the final Item.
func (bb *BareItemBuilder[B, T]) ToItem() *ItemBuilder {
	if bb.err != nil {
		return &ItemBuilder{err: bb.err}
	}

	return &ItemBuilder{
		value: bb.value.ToItem(),
	}
}

// Value sets the value for the BareItem being built. Returns the same
// BareItemBuilder for method chaining. If an error occurs during value
// setting, it will be recorded and returned by Build().
func (bb *BareItemBuilder[B, T]) Value(value T) *BareItemBuilder[B, T] {
	if bb.err != nil {
		return bb
	}
	if err := bb.setter(value); err != nil {
		bb.err = fmt.Errorf("error setting value: %w", err)
	}
	return bb
}

// Build constructs and returns the BareItem. Returns an error if any
// step in the building process failed.
func (bb *BareItemBuilder[B, T]) Build() (B, error) {
	if bb.err != nil {
		var zero B
		return zero, bb.err
	}
	return bb.value, nil
}

// MustBuild constructs and returns the BareItem, panicking if any error
// occurred during the building process. Use this when you are confident
// that the building process will succeed.
func (bb *BareItemBuilder[B, T]) MustBuild() B {
	if bb.err != nil {
		panic(bb.err)
	}
	return bb.value
}

// Parameter adds a parameter to the item being built, and in the meanwhile
// upgrades the builder to an ItemBuilder from a BareItemBuilder.
// If there was an error in the builder to start with, the error will be
// carried over to the ItemBuilder (and the building process will not continue
// internally).
func (bb *BareItemBuilder[B, T]) Parameter(k string, v BareItem) *ItemBuilder {
	var ib ItemBuilder
	if bb.err != nil {
		ib.err = bb.err
		return &ib
	}

	ib.value = bb.value.ToItem()
	if err := ib.value.Parameters().Set(k, v); err != nil {
		ib.err = fmt.Errorf("error setting parameter %q: %w", k, err)
		return &ib
	}

	return &ib
}

// Build constructs and returns the Item. Returns an error if any
// step in the building process failed.
func (ib *ItemBuilder) Build() (Item, error) {
	if ib.err != nil {
		return nil, ib.err
	}
	return ib.value, nil
}

// MustBuild constructs and returns the Item, panicking if any error
// occurred during the building process. Use this when you are confident
// that the building process will succeed.
func (ib *ItemBuilder) MustBuild() Item {
	if ib.err != nil {
		panic(ib.err)
	}
	return ib.value
}

// Parameter adds a parameter to the Item being built. Returns the same
// ItemBuilder for method chaining. If an error occurs during parameter
// setting, it will be recorded and returned by Build().
func (ib *ItemBuilder) Parameter(k string, v BareItem) *ItemBuilder {
	if ib.err != nil {
		return ib
	}
	if err := ib.value.Parameters().Set(k, v); err != nil {
		ib.err = fmt.Errorf("error setting parameter %q: %w", k, err)
	}
	return ib
}
