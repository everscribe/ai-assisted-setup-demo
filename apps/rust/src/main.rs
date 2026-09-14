// The store is the whole crate today. Task 5 adds the server that calls it,
// so drop this allow once the handlers land.
#![allow(dead_code)]

mod memory;
mod models;
mod seed;
mod store;

fn main() {
    println!("Hello, world!");
}
