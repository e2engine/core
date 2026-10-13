// Package matcher defines declarative matchers used by E2Engine specifications.
//
// Matchers have a compact scalar form for exact equality and an expanded object
// form for other operators. For example, the YAML/JSON string `"foo"` is
// unmarshaled as String{Operator: StringMatchOperatorEquals, Value: "foo"},
// while {"operator":"nonempty"} is unmarshaled as a nonempty matcher. Exact
// matchers are marshaled back to their compact scalar form; other operators use
// the expanded form. GRPCMessage follows the same convention, with its exact
// value represented as a JSON string containing the protobuf message.
package matcher
