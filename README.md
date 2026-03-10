## Bittorrent client

This is learning project, where i build a bittorrent taking the lead from Jesse Li's Building a BitTorrent client from the ground up in Go

### Bencode

Bencode (short for binary encoding) is a very simple data serialization format used mainly by the BitTorrent protocol.

In BitTorrent, bencode is used to store and transmit information such as:

- .torrent file metadata
- tracker responses
- peer communication data

Bencode only supports four types of values.

- integers: Integers are written with i at the start and e at the end.
  example: i42e is 42 and i-5e is -5, the structure is i<number>e

- strings: they are written as length:string
  example: 4:spam is "spam", 11:hello world is "hello world"

- lists: Lists start with l and end with e.
  example: l4:spam4:eggse is ["spam", "eggs"], li1ei2ei3ee is [1, 2, 3], the structure is l <item1> <item2> ... e

- dictionaries: Dictionaries start with d and end with e
  example: d3:cow3:moo4:spam4:eggse is {
  "cow": "moo",
  "spam": "eggs"
  }, structure is d <key> <value> <key> <value> e

bencode.Unmarshal(r, &bto)

```

It reads the raw bencode bytes from the `.torrent` file and fills in your `bencodeTorrent` struct automatically — matching each bencode key to the correct struct field using those struct tags.

Think of it like this:
```

raw .torrent file bytes → Unmarshal → bencodeTorrent struct

### bittorent handshake

- The length of the protocol identifier, which is always 19 (0x13 in hex)
- The protocol identifier, called the pstr which is always BitTorrent protocol
- Eight reserved bytes, all set to 0. We’d flip some of them to 1 to indicate that we support certain extensions. But we don’t, so we’ll keep them at 0.
- The infohash that we calculated earlier to identify which file we want
- The Peer ID that we made up to identify ourselves

| ID  | Name       | Meaning                  |
| --- | ---------- | ------------------------ |
| 0   | Choke      | Peer won't send you data |
| 1   | Unchoke    | Peer will send you data  |
| 2   | Interested | You want data            |
| 4   | Have       | Peer has a piece         |
| 5   | Bitfield   | Which pieces peer has    |
| 6   | Request    | Ask for a block          |
| 7   | Piece      | Actual data              |
