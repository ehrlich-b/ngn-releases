"""Bounded reads of an immutable archive held on another campaign host."""

import json
from pathlib import Path
from urllib.request import Request, urlopen

import numpy as np


class RemoteArray:
    """Array-like record access using one exact HTTP byte range per read.

    The descriptor pins the file's size and SHA-256. The serving host verifies
    that digest when publishing the descriptor and uses it as the HTTP ETag.
    Training indexes stay within one archive block, so no corpus-size cache or
    local copy is needed. Errors fail the read rather than yielding partial rows.
    """

    def __init__(self, descriptor, dtype):
        self.descriptor = json.loads(Path(descriptor).read_text())
        self.dtype = dtype
        self.size = self.descriptor["bytes"]
        self.url = self.descriptor["url"]
        digest = self.descriptor["sha256"]
        if (self.size <= 0 or self.size % dtype.itemsize
                or len(digest) != 64 or any(c not in "0123456789abcdef" for c in digest)
                or not self.url.startswith(("http://", "https://"))):
            raise ValueError("Invalid remote archive descriptor")
        self.etag = '"' + digest + '"'
        self.count = self.size // dtype.itemsize

    def __getitem__(self, indexes):
        if isinstance(indexes, slice):
            start, end, step = indexes.indices(self.count)
            if step != 1:
                raise ValueError("Remote archive slices must be contiguous")
            if end <= start:
                return np.empty(0, dtype=self.dtype)
            relative = slice(None)
        else:
            indexes = np.asarray(indexes)
            if indexes.ndim != 1 or indexes.dtype.kind not in "iu":
                raise ValueError("Remote archive indexes must be a one-dimensional integer array")
            if not len(indexes):
                return np.empty(0, dtype=self.dtype)
            start, end = int(indexes.min()), int(indexes.max()) + 1
            if start < 0 or end > self.count:
                raise IndexError("Remote archive index outside file")
            relative = indexes - start
        first, last = start * self.dtype.itemsize, end * self.dtype.itemsize - 1
        request = Request(self.url, headers={"Range": f"bytes={first}-{last}", "If-Match": self.etag})
        with urlopen(request, timeout=60) as response:
            if (response.status != 206 or response.headers.get("ETag") != self.etag
                    or response.headers.get("Content-Range") != f"bytes {first}-{last}/{self.size}"
                    or response.headers.get("Content-Length") != str(last - first + 1)):
                raise ValueError("Remote archive response differs from pinned byte range")
            raw = response.read(last - first + 2)
        if len(raw) != last - first + 1:
            raise ValueError("Incomplete remote archive byte range")
        return np.frombuffer(raw, dtype=self.dtype)[relative]
