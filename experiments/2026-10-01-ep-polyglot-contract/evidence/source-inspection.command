{ nl -ba engine/polyglot.go | sed -n '24,70p'; nl -ba engine/book_test.go | sed -n '68,84p'; } | sed -E 's/[[:blank:]]+$//'
