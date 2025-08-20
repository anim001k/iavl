# IAVL v2

https://www.youtube.com/watch?v=keV22tP8nks

## Node Key Format

Node keys are 12-byte arrays with the following structure:

- 8 bytes for version (big-endian uint64)
- 4 bytes for sequence (big-endian uint32)
    - sequence numbers are unique within a version and incremented for each new node
    - leaf node sequences have the high bit set (>= 0x80000000) to distinguish them from internal nodes
    - leaf node sequence numbers are assigned both for insertion and deletions, in the order of operations (so that they
      can be used to reconstruct the tree)

## Insertion Algorithm

## Database Structure

* leaf nodes and internal nodes are stored in separate tables
* leaf nodes are stored in the `leaf` table
* internal nodes are stored in `tree_{version}` tables, where `{version}` is the version of the tree, but these trees
  are only present for versions that are checkpoints (TODO: verify this)

### Database Files

#### `changelog.sqlite`

Tables:

* `latest`
* `leaf`
* `leaf_delete`
* `leaf_orphan`
* `snapshot_{version}` (for each snapshot version)

#### `tree.sqlite`

Tables:

* `root`
* `tree_{version}` (for each checkpoint version)
* `orphan`

### `root` table

Columns:

- `version`: int, the version of the tree
- `node_version`: int, the version of the node
- `node_sequence`: int, the sequence number of the node (unique within the version)
- `bytes`: blob, encoding?? TODO
- `checkpoint`: bool, whether this version is a checkpoint
- `PRIMARY KEY (version)`: ensures each version is unique

### `tree_{version}` and `leaf` tables

Columns:

- `version`: int, the version of the tree
- `sequence`: int, the sequence number of the leaf node insertion (unique within the version)
- `bytes`: blob, see [encoding below](#node-bytes-encoding)
- `orphaned`: bool, whether the leaf node is orphaned (TODO)

### `leaf_delete` table

Columns:

- version: int, the version of the tree
- sequence: int, the sequence number of the leaf node deletion (unique within the version)
- key: bytes, the key of the leaf node that was deleted

### `leaf` and `leaf_delete` tables functions as changelog

The `leaf` and `leaf_delete` tables function as a changelog for the tree.
Given a tree at a given checkpoint (or genesis), the entries in the `leaf` and `leaf_delete` tables can be in
sequence order to reconstruct the tree at any target version.
Recall that the hash of a tree is dependent on insertion order.
Because sequence numbers are assigned in order of insertion/deletion within a version,
they can be used to accurately reconstruct the tree at any point in time.

### Node `bytes` encoding

In `leaf` table and `tree_{version}` tables, the `bytes` field is encoded as follows:

* `height` varint, `int8` max range
* `size` varint
* `key` bytes (varint length prefixed)
* `hash` bytes (varint length prefixed)
* if leaf node:
    * `value` bytes (varint length prefixed)
* else (internal node):
    * `leftNodeKey` (varint length prefixed but always 12 bytes)
    * `rightNodeKey` (varint length prefixed but always 12 bytes)

## Loading a Version

* get list of versions which are marked as checkpoints
* 