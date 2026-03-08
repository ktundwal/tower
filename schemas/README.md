# Tower schemas

This directory holds versioned schemas for the normalized data Tower emits and stores.

The bootstrap seeds the event-envelope schema first because it anchors adapters, fixture replay, and future SQLite persistence around one shared contract. Additional schemas for parked bundles, audit exports, and fixture families should be added here as those slices land.
