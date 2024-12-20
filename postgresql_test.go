package lucene

import (
	"strings"
	"testing"
)

func TestPostgresSQLEndToEnd(t *testing.T) {
	type tc struct {
		input        string
		want         string
		defaultField string
		err          string
	}

	tcs := map[string]tc{
		// "single_literal": {
		// 	input: "a",
		// 	want:  `a`,
		// },
		"basic_equal": {
			input: "a:b",
			want:  `"a" = 'b'`,
		},
		"basic_equal_with_number": {
			input: "a:5",
			want:  `"a" = 5`,
		},
		"basic_greater_with_number": {
			input: "a:>22",
			want:  `"a" > 22`,
		},
		"basic_greater_eq_with_number": {
			input: "a:>=22",
			want:  `"a" >= 22`,
		},
		"basic_less_with_number": {
			input: "a:<22",
			want:  `"a" < 22`,
		},
		"basic_less_eq_with_number": {
			input: "a:<=22",
			want:  `"a" <= 22`,
		},
		"basic_greater_less_with_number": {
			input: "a:<22 AND b:>33",
			want:  `("a" < 22) AND ("b" > 33)`,
		},
		"basic_greater_less_eq_with_number": {
			input: "a:<=22 AND b:>=33",
			want:  `("a" <= 22) AND ("b" >= 33)`,
		},
		"basic_wild_equal_with_*": {
			input: "a:b*",
			want:  `"a" SIMILAR TO 'b%'`,
		},
		"basic_wild_equal_with_?": {
			input: "a:b?z",
			want:  `"a" SIMILAR TO 'b_z'`,
		},
		"basic_inclusive_range": {
			input: "a:[* TO 5]",
			want:  `"a" <= 5`,
		},
		"basic_exclusive_range": {
			input: "a:{* TO 5}",
			want:  `"a" < 5`,
		},
		"range_over_strings": {
			input: "a:{foo TO bar}",
			want:  `"a" BETWEEN 'foo' AND 'bar'`,
		},
		"basic_fuzzy": {
			input: "b AND a~",
			err:   "unable to render operator [FUZZY]",
		},
		"fuzzy_power": {
			input: "b AND a~10",
			err:   "unable to render operator [FUZZY]",
		},
		"basic_boost": {
			input: "b AND a^",
			err:   "unable to render operator [BOOST]",
		},
		"boost_power": {
			input: "b AND a^10",
			err:   "unable to render operator [BOOST]",
		},
		"regexp": {
			input: "a:/b [c]/",
			want:  `"a" ~ '/b [c]/'`,
		},
		"regexp_with_keywords": {
			input: `a:/b "[c]/`,
			want:  `"a" ~ '/b "[c]/'`,
		},
		"regexp_with_escaped_chars": {
			input: `url:/example.com\/foo\/bar\/.*/`,
			want:  `"url" ~ '/example.com\/foo\/bar\/.*/'`,
		},
		"basic_default_AND": {
			input: "a b",
			want:  `'a' AND 'b'`,
		},
		"default_to_AND_with_subexpressions": {
			input: "a:b c:d",
			want:  `("a" = 'b') AND ("c" = 'd')`,
		},
		"basic_and": {
			input: "a AND b",
			want:  `'a' AND 'b'`,
		},
		"and_with_nesting": {
			input: "a:foo AND b:bar",
			want:  `("a" = 'foo') AND ("b" = 'bar')`,
		},
		"basic_or": {
			input: "a OR b",
			want:  `'a' OR 'b'`,
		},
		"or_with_nesting": {
			input: "a:foo OR b:bar",
			want:  `("a" = 'foo') OR ("b" = 'bar')`,
		},
		"range_operator_inclusive": {
			input: "a:[1 TO 5]",
			want:  `"a" >= 1 AND "a" <= 5`,
		},
		"range_operator_inclusive_unbound": {
			input: `a:[* TO 200]`,
			want:  `"a" <= 200`,
		},
		"range_operator_exclusive": {
			input: `a:{"ab" TO "az"}`,
			want:  `"a" BETWEEN 'ab' AND 'az'`,
		},
		"range_operator_exclusive_unbound": {
			input: `a:{2 TO *}`,
			want:  `"a" > 2`,
		},
		"basic_not": {
			input: "NOT b",
			want:  `NOT('b')`,
		},
		"nested_not": {
			input: "a:foo OR NOT b:bar",
			want:  `("a" = 'foo') OR (NOT("b" = 'bar'))`,
		},
		"term_grouping": {
			input: "(a:foo OR b:bar) AND c:baz",
			want:  `(("a" = 'foo') OR ("b" = 'bar')) AND ("c" = 'baz')`,
		},
		"value_grouping": {
			input: "a:(foo OR baz OR bar)",
			want:  `"a" IN ('foo', 'baz', 'bar')`,
		},
		"basic_must": {
			input: "+a:b",
			want:  `"a" = 'b'`,
		},
		"basic_must_not": {
			input: "-a:b",
			want:  `NOT("a" = 'b')`,
		},
		"basic_nested_must_not": {
			input: "d:e AND (-a:b AND +f:e)",
			want:  `("d" = 'e') AND ((NOT("a" = 'b')) AND ("f" = 'e'))`,
		},
		"basic_escaping": {
			input: `a:\(1\+1\)\:2`,
			want:  `"a" = '(1+1):2'`,
		},
		"escaped_column_name": {
			input: `foo\ bar:b`,
			want:  `"foo bar" = 'b'`,
		},
		"boost_key_value": {
			input: "a:b^2 AND foo",
			err:   "unable to render operator [BOOST]",
		},
		"nested_sub_expressions": {
			input: "((title:foo OR title:bar) AND (body:foo OR body:bar)) OR k:v",
			want:  `((("title" = 'foo') OR ("title" = 'bar')) AND (("body" = 'foo') OR ("body" = 'bar'))) OR ("k" = 'v')`,
		},
		"fuzzy_key_value": {
			input: "a:b~2 AND foo",
			err:   "unable to render operator [FUZZY]",
		},
		"precedence_works": {
			input: "a:b AND c:d OR e:f OR h:i AND j:k",
			want:  `((("a" = 'b') AND ("c" = 'd')) OR ("e" = 'f')) OR (("h" = 'i') AND ("j" = 'k'))`,
		},
		"test_precedence_weaving": {
			input: "a OR b AND c OR d",
			want:  `('a' OR ('b' AND 'c')) OR 'd'`,
		},
		"test_precedence_weaving_with_not": {
			input: "NOT a OR b AND NOT c OR d",
			want:  `((NOT('a')) OR ('b' AND (NOT('c')))) OR 'd'`,
		},
		"test_equals_in_precedence": {
			input: "a:az OR b:bz AND NOT c:z OR d",
			want:  `(("a" = 'az') OR (("b" = 'bz') AND (NOT("c" = 'z')))) OR 'd'`,
		},
		"test_parens_in_precedence": {
			input: "a AND (c OR d)",
			want:  `'a' AND ('c' OR 'd')`,
		},
		"test_range_precedence_simple": {
			input: "c:[* to -1] OR d",
			want:  `("c" <= -1) OR 'd'`,
		},
		"test_range_precedence": {
			input: "a OR b AND c:[* to -1] OR d",
			want:  `('a' OR ('b' AND ("c" <= -1))) OR 'd'`,
		},
		"test_full_precedence": {
			input: "a OR b AND c:[* to -1] OR d AND NOT +e:f",
			want:  `('a' OR ('b' AND ("c" <= -1))) OR ('d' AND (NOT("e" = 'f')))`,
		},
		"test_elastic_greater_than_precedence": {
			input: "a:>10 AND -b:<=-20",
			want:  `("a" > 10) AND (NOT("b" <= -20))`,
		},
		"escape_quotes": {
			input: "a:'b'",
			want:  `"a" = '''b'''`,
		},
		"name_starts_with_number": {
			input: "1a:b",
			want:  `"1a" = 'b'`,
		},
		"default_field_and": {
			input:        `title:"The Right Way" AND go`,
			want:         `("title" = 'The Right Way') AND ("default" = 'go')`,
			defaultField: "default",
		},
		"default_field_or": {
			input:        `title:"The Right Way" OR go`,
			want:         `("title" = 'The Right Way') OR ("default" = 'go')`,
			defaultField: "default",
		},
		"default_field_not": {
			input:        `title:"The Right Way" AND NOT(go)`,
			want:         `("title" = 'The Right Way') AND (NOT("default" = 'go'))`,
			defaultField: "default",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			got, err := ToPostgres(tc.input, WithDefaultField(tc.defaultField))
			if err != nil {
				// if we got an expect error then we are fine
				if tc.err != "" && strings.Contains(err.Error(), tc.err) {
					return
				}
				t.Fatalf("unexpected error rendering expression: %v", err)
			}

			if tc.err != "" {
				t.Fatalf("\nexpected error [%s]\ngot: %s", tc.err, got)
			}

			if got != tc.want {
				expr, err := Parse(tc.input)
				if err != nil {
					t.Fatalf("unable to parse expression: %v", err)
				}
				t.Fatalf("\nwant %s\ngot  %s\nparsed expression: %#v\n", tc.want, got, expr)
			}
		})
	}
}

func TestPostgresParameterizedSQLEndToEnd(t *testing.T) {
	type tc struct {
		input        string
		wantStr      string
		wantParams   []any
		defaultField string
		err          string
	}

	tcs := map[string]tc{
		// "single_literal": {
		// 	input: "a",
		// 	want:  `a`,
		// },
		"basic_equal": {
			input:      "a:b",
			wantStr:    `"a" = ?`,
			wantParams: []any{"b"},
		},
		"basic_equal_with_number": {
			input:      "a:5",
			wantStr:    `"a" = ?`,
			wantParams: []any{5},
		},
		"basic_greater_with_number": {
			input:      "a:>22",
			wantStr:    `"a" > ?`,
			wantParams: []any{22},
		},
		"basic_greater_eq_with_number": {
			input:      "a:>=22",
			wantStr:    `"a" >= ?`,
			wantParams: []any{22},
		},
		"basic_less_with_number": {
			input:      "a:<22",
			wantStr:    `"a" < ?`,
			wantParams: []any{22},
		},
		"basic_less_eq_with_number": {
			input:      "a:<=22",
			wantStr:    `"a" <= ?`,
			wantParams: []any{22},
		},
		"basic_greater_less_with_number": {
			input:      "a:<22 AND b:>33",
			wantStr:    `("a" < ?) AND ("b" > ?)`,
			wantParams: []any{22, 33},
		},
		"basic_greater_less_eq_with_number": {
			input:      "a:<=22 AND b:>=33",
			wantStr:    `("a" <= ?) AND ("b" >= ?)`,
			wantParams: []any{22, 33},
		},
		"basic_wild_equal_with_*": {
			input:      "a:b*",
			wantStr:    `"a" SIMILAR TO ?`,
			wantParams: []any{"b%"},
		},
		"basic_wild_equal_with_?": {
			input:      "a:b?z",
			wantStr:    `"a" SIMILAR TO ?`,
			wantParams: []any{"b_z"},
		},
		"basic_inclusive_range": {
			input:      "a:[* TO 5]",
			wantStr:    `"a" <= ?`,
			wantParams: []any{5},
		},
		"basic_exclusive_range": {
			input:      "a:{* TO 5}",
			wantStr:    `"a" < ?`,
			wantParams: []any{5},
		},
		"range_over_strings": {
			input:      "a:{foo TO bar}",
			wantStr:    `"a" BETWEEN ? AND ?`,
			wantParams: []any{"foo", "bar"},
		},
		"basic_fuzzy": {
			input: "b AND a~",
			err:   "unable to render operator [FUZZY]",
		},
		"fuzzy_power": {
			input: "b AND a~10",
			err:   "unable to render operator [FUZZY]",
		},
		"basic_boost": {
			input: "b AND a^",
			err:   "unable to render operator [BOOST]",
		},
		"boost_power": {
			input: "b AND a^10",
			err:   "unable to render operator [BOOST]",
		},
		"regexp": {
			input:      "a:/b [c]/",
			wantStr:    `"a" ~ ?`,
			wantParams: []any{"/b [c]/"},
		},
		"regexp_with_keywords": {
			input:      `a:/b "[c]/`,
			wantStr:    `"a" ~ ?`,
			wantParams: []any{`/b "[c]/`},
		},
		"regexp_with_escaped_chars": {
			input:      `url:/example.com\/foo\/bar\/.*/`,
			wantStr:    `"url" ~ ?`,
			wantParams: []any{`/example.com\/foo\/bar\/.*/`},
		},
		"basic_default_AND": {
			input:      "a b",
			wantStr:    `? AND ?`,
			wantParams: []any{"a", "b"},
		},
		"default_to_AND_with_subexpressions": {
			input:      "a:b c:d",
			wantStr:    `("a" = ?) AND ("c" = ?)`,
			wantParams: []any{"b", "d"},
		},
		"basic_and": {
			input:      "a AND b",
			wantStr:    `? AND ?`,
			wantParams: []any{"a", "b"},
		},
		"and_with_nesting": {
			input:      "a:foo AND b:bar",
			wantStr:    `("a" = ?) AND ("b" = ?)`,
			wantParams: []any{"foo", "bar"},
		},
		"basic_or": {
			input:      "a OR b",
			wantStr:    `? OR ?`,
			wantParams: []any{"a", "b"},
		},
		"or_with_nesting": {
			input:      "a:foo OR b:bar",
			wantStr:    `("a" = ?) OR ("b" = ?)`,
			wantParams: []any{"foo", "bar"},
		},
		"range_operator_inclusive": {
			input:      "a:[1 TO 5]",
			wantStr:    `"a" >= ? AND "a" <= ?`,
			wantParams: []any{1, 5},
		},
		"range_operator_inclusive_unbound": {
			input:      `a:[* TO 200]`,
			wantStr:    `"a" <= ?`,
			wantParams: []any{200},
		},
		"range_operator_exclusive": {
			input:      `a:{"ab" TO "az"}`,
			wantStr:    `"a" BETWEEN ? AND ?`,
			wantParams: []any{"ab", "az"},
		},
		"range_operator_exclusive_unbound": {
			input:      `a:{2 TO *}`,
			wantStr:    `"a" > ?`,
			wantParams: []any{2},
		},
		"basic_not": {
			input:      "NOT b",
			wantStr:    `NOT(?)`,
			wantParams: []any{"b"},
		},
		"nested_not": {
			input:      "a:foo OR NOT b:bar",
			wantStr:    `("a" = ?) OR (NOT("b" = ?))`,
			wantParams: []any{"foo", "bar"},
		},
		"term_grouping": {
			input:      "(a:foo OR b:bar) AND c:baz",
			wantStr:    `(("a" = ?) OR ("b" = ?)) AND ("c" = ?)`,
			wantParams: []any{"foo", "bar", "baz"},
		},
		"value_grouping": {
			input:      "a:(foo OR baz OR bar)",
			wantStr:    `"a" IN (?, ?, ?)`,
			wantParams: []any{"foo", "baz", "bar"},
		},
		"basic_must": {
			input:      "+a:b",
			wantStr:    `"a" = ?`,
			wantParams: []any{"b"},
		},
		"basic_must_not": {
			input:      "-a:b",
			wantStr:    `NOT("a" = ?)`,
			wantParams: []any{"b"},
		},
		"basic_nested_must_not": {
			input:      "d:e AND (-a:b AND +f:e)",
			wantStr:    `("d" = ?) AND ((NOT("a" = ?)) AND ("f" = ?))`,
			wantParams: []any{"e", "b", "e"},
		},
		"basic_escaping": {
			input:      `a:\(1\+1\)\:2`,
			wantStr:    `"a" = ?`,
			wantParams: []any{"(1+1):2"},
		},
		"escaped_column_name": {
			input:      `foo\ bar:b`,
			wantStr:    `"foo bar" = ?`,
			wantParams: []any{"b"},
		},
		"boost_key_value": {
			input: "a:b^2 AND foo",
			err:   "unable to render operator [BOOST]",
		},
		"nested_sub_expressions": {
			input:      "((title:foo OR title:bar) AND (body:foo OR body:bar)) OR k:v",
			wantStr:    `((("title" = ?) OR ("title" = ?)) AND (("body" = ?) OR ("body" = ?))) OR ("k" = ?)`,
			wantParams: []any{"foo", "bar", "foo", "bar", "v"},
		},
		"fuzzy_key_value": {
			input: "a:b~2 AND foo",
			err:   "unable to render operator [FUZZY]",
		},
		"precedence_works": {
			input:      "a:b AND c:d OR e:f OR h:i AND j:k",
			wantStr:    `((("a" = ?) AND ("c" = ?)) OR ("e" = ?)) OR (("h" = ?) AND ("j" = ?))`,
			wantParams: []any{"b", "d", "f", "i", "k"},
		},
		"test_precedence_weaving": {
			input:      "a OR b AND c OR d",
			wantStr:    `(? OR (? AND ?)) OR ?`,
			wantParams: []any{"a", "b", "c", "d"},
		},
		"test_precedence_weaving_with_not": {
			input:      "NOT a OR b AND NOT c OR d",
			wantStr:    `((NOT(?)) OR (? AND (NOT(?)))) OR ?`,
			wantParams: []any{"a", "b", "c", "d"},
		},
		"test_equals_in_precedence": {
			input:      "a:az OR b:bz AND NOT c:z OR d",
			wantStr:    `(("a" = ?) OR (("b" = ?) AND (NOT("c" = ?)))) OR ?`,
			wantParams: []any{"az", "bz", "z", "d"},
		},
		"test_parens_in_precedence": {
			input:      "a AND (c OR d)",
			wantStr:    `? AND (? OR ?)`,
			wantParams: []any{"a", "c", "d"},
		},
		"test_range_precedence_simple": {
			input:      "c:[* to -1] OR d",
			wantStr:    `("c" <= ?) OR ?`,
			wantParams: []any{-1, "d"},
		},
		"test_range_precedence": {
			input:      "a OR b AND c:[* to -1] OR d",
			wantStr:    `(? OR (? AND ("c" <= ?))) OR ?`,
			wantParams: []any{"a", "b", -1, "d"},
		},
		"test_full_precedence": {
			input:      "a OR b AND c:[* to -1] OR d AND NOT +e:f",
			wantStr:    `(? OR (? AND ("c" <= ?))) OR (? AND (NOT("e" = ?)))`,
			wantParams: []any{"a", "b", -1, "d", "f"},
		},
		"test_elastic_greater_than_precedence": {
			input:      "a:>10 AND -b:<=-20",
			wantStr:    `("a" > ?) AND (NOT("b" <= ?))`,
			wantParams: []any{10, -20},
		},
		"escape_quotes": {
			input:      "a:'b'",
			wantStr:    `"a" = ?`,
			wantParams: []any{"'b'"},
		},
		"name_starts_with_number": {
			input:      "1a:b",
			wantStr:    `"1a" = ?`,
			wantParams: []any{"b"},
		},
		"default_field_and": {
			input:        `title:"The Right Way" AND go`,
			wantStr:      `("title" = ?) AND ("default" = ?)`,
			wantParams:   []any{"The Right Way", "go"},
			defaultField: "default",
		},
		"default_field_or": {
			input:        `title:"The Right Way" OR go`,
			wantStr:      `("title" = ?) OR ("default" = ?)`,
			wantParams:   []any{"The Right Way", "go"},
			defaultField: "default",
		},
		"default_field_not": {
			input:        `title:"The Right Way" AND NOT(go)`,
			wantStr:      `("title" = ?) AND (NOT("default" = ?))`,
			wantParams:   []any{"The Right Way", "go"},
			defaultField: "default",
		},
		"default_bare_field": {
			input:        `this is an example`,
			wantStr:      `((("default" = ?) AND ("default" = ?)) AND ("default" = ?)) AND ("default" = ?)`,
			wantParams:   []any{"this", "is", "an", "example"},
			defaultField: "default",
		},
		"default_single_literal": {
			input:        `a`,
			wantStr:      `"default" = ?`,
			wantParams:   []any{"a"},
			defaultField: "default",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			gotStr, gotParams, err := ToParameterizedPostgres(tc.input, WithDefaultField(tc.defaultField))
			if err != nil {
				// if we got an expect error then we are fine
				if tc.err != "" && strings.Contains(err.Error(), tc.err) {
					return
				}
				t.Fatalf("unexpected error rendering expression: %v", err)
			}

			if tc.err != "" {
				t.Fatalf("\nexpected error [%s]\ngot: %s", tc.err, gotStr)
			}

			if gotStr != tc.wantStr {
				expr, err := Parse(tc.input)
				if err != nil {
					t.Fatalf("unable to parse expression: %v", err)
				}
				t.Fatalf("\nwant %s\ngot  %s\nparsed expression: %#v\n", tc.wantStr, gotStr, expr)
			}

			if len(gotParams) != len(tc.wantParams) {
				t.Fatalf("expected %d params(%v), got %d (%v)", len(tc.wantParams), tc.wantParams, len(gotParams), gotParams)
			}

			for i := range gotParams {
				if gotParams[i] != tc.wantParams[i] {
					t.Fatalf("expected param %d to be %v, got %v", i, tc.wantParams[i], gotParams[i])
				}
			}
		})
	}
}
