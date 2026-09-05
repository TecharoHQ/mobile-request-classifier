.PHONY: rust

WASM_FEATURES := --enable-bulk-memory --enable-sign-ext --enable-nontrapping-float-to-int --enable-mutable-globals

gravity:
	rm phone_classifier/phone_classifier.go
	gravity -w classifier ./target/wasm32-unknown-unknown/release/mobile_request_classifier.wasm > phone_classifier/phone_classifier.go
	go tool goimports phone_classifier/phone_classifier2.go
	patch ./phone_classifier/phone_classifier2.go ./phone_classifier/hack.patch 

rust:
	cargo build --target wasm32-unknown-unknown --release
	wasm-opt -O3 $(WASM_FEATURES) --strip-debug --strip-producers ./target/wasm32-unknown-unknown/release/mobile_request_classifier.wasm -o ./var/mobile_request_classifier.wasm