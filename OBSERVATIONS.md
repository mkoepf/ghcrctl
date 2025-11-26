# Performance / usability 

## TYPE column inconsitencies

The TYPE column in the version command in inconsitently used.

Example 1: Some are "untagged" which is actually not a type. It is also redundant, as untagged simply means: [] in TAGS column.

  VERSION ID  TYPE              DIGEST        TAGS                                                      CREATED
  ----------  ----------------  ------------  --------------------------------------------------------  -------------------
┌ 588641743   index             3be2c003c77e  [newest, classic-pat-test, stable, test-1, v1.0, latest]  2025-11-25 12:10:31
├ 588641725   linux/amd64       f81014ec9fd5  []                                                        2025-11-25 12:10:30
├ 588641732   linux/arm64       fee8e7f7815d  []                                                        2025-11-25 12:10:30
├ 588641716   sbom, provenance  1a0e6756f320  []                                                        2025-11-25 12:10:29
└ 588641739   provenance, sbom  d27cc69ebd00  []                                                        2025-11-25 12:10:30

┌ 588579607   untagged          6aea791a2d8d  []                                                        2025-11-25 11:19:07
└ 588579577   linux/amd64       f0406fcc380e  []                                                        2025-11-25 11:19:06

┌ 585861918   untagged          01af50cc8b0d  []                                                        2025-11-22 15:41:38
├ 585861916   linux/amd64       62f946a8267d  []                                                        2025-11-22 15:41:37
└ 585861917   sbom, provenance  9a1636d22702  []                                                        2025-11-22 15:41:37

Example 2: Some versions are sometimes shown as untagged, sometimes as different types.  Look at DIGEST 1a0e6756f320 in these two different filtered views:

./ghcrctl versions ghcrctl-test-with-sbom --tagged
Versions for ghcrctl-test-with-sbom:

  VERSION ID  TYPE              DIGEST        TAGS                                                      CREATED
  ----------  ----------------  ------------  --------------------------------------------------------  -------------------
┌ 588641743   index             3be2c003c77e  [newest, classic-pat-test, stable, test-1, v1.0, latest]  2025-11-25 12:10:31
├ 588641739   sbom, provenance  d27cc69ebd00  []                                                        2025-11-25 12:10:30
├ 588641725   linux/amd64       f81014ec9fd5  []                                                        2025-11-25 12:10:30
├ 588641732   linux/arm64       fee8e7f7815d  []                                                        2025-11-25 12:10:30
└ 588641716   provenance, sbom  1a0e6756f320  []                                                        2025-11-25 12:10:29

./ghcrctl versions ghcrctl-test-with-sbom --untagged
Warning: failed to determine attestation type for sha256:18a6ae3b2b10a3f9e649cd8de3b434bfc0daecda644b2ed00bcd6f944d5b1369: failed to fetch manifest: sha256:18a6ae3b2b10a3f9e649cd8de3b434bfc0daecda644b2ed00bcd6f944d5b1369: not found
Warning: failed to determine attestation type for sha256:cca2d3fbaac0f656634786844f9e002230d23100b84c4201c9f53c44392130bc: failed to fetch manifest: sha256:cca2d3fbaac0f656634786844f9e002230d23100b84c4201c9f53c44392130bc: not found
Versions for ghcrctl-test-with-sbom:

  VERSION ID  TYPE              DIGEST        TAGS  CREATED
  ----------  ----------------  ------------  ----  -------------------
┌ 588579607   untagged          6aea791a2d8d  []    2025-11-25 11:19:07
└ 588579577   linux/amd64       f0406fcc380e  []    2025-11-25 11:19:06

┌ 585861918   untagged          01af50cc8b0d  []    2025-11-22 15:41:38
├ 585861916   linux/amd64       62f946a8267d  []    2025-11-22 15:41:37
└ 585861917   provenance, sbom  9a1636d22702  []    2025-11-22 15:41:37

  588641739   untagged          d27cc69ebd00  []    2025-11-25 12:10:30
  588641732   untagged          fee8e7f7815d  []    2025-11-25 12:10:30
  588641725   untagged          f81014ec9fd5  []    2025-11-25 12:10:30
  588641716   untagged          1a0e6756f320  []    2025-11-25 12:10:29

## Delete behavior when deleting single package versions

When an attestation version is deleted from a graph, then ghcrctl triggers a warning (issued by oras or gh api?):
```
Warning: failed to determine attestation type for sha256:18a6ae3b2b10a3f9e649cd8de3b434bfc0daecda644b2ed00bcd6f944d5b1369: failed to fetch manifest: sha256:18a6ae3b2b10a3f9e649cd8de3b434bfc0daecda644b2ed00bcd6f944d5b1369: not found
```
The deleted attestations are still shown, but as unknown:
```
Image Index: sha256:6aea791a2d8d2795505edc2ee029c7d8f2b76cff34912259b55fe0ad94d612c0
  Tags: [newest, classic-pat-test, stable, test-1, v1.0, latest]
  Version ID: 588579607
  │
  ├─ Platform Manifests (references):
  │    ├─ linux/amd64
  │       Digest: sha256:f0406fcc380e...
  │       Size: 669 bytes
  │    └─ linux/arm64
  │       Digest: sha256:3cc4528291f7...
  │       Size: 669 bytes
  │
  └─ Attestations (referrers):
         ├─ unknown
            Digest: sha256:18a6ae3b2b10...
         └─ unknown
            Digest: sha256:cca2d3fbaac0...
```


# Delete behavior of ghcr / github packages

I am doing:

> docker pull ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest
> docker buildx imagetools inspect ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest --format "{{json .SBOM}}
> docker buildx imagetools inspect ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest --format "{{json .Provenance}}"

Both, SBOM and Provenance are present.

Then I get the SBOM/Provenance version-ids (two, because there are two arch manifests) using

> ./ghcrctl versions ghcrctl-test-with-sbom

Then I delete these versions:

> ./ghcrctl delete ghcrctl-test-with-sbom <second versionid>
> ./ghcrctl delete ghcrctl-test-with-sbom <first versionid>


I delete the previously pulled image:

> docker rmi ghcr.io/mkoepf/ghcrctl-test-with-sbom

Then, again:

> docker pull ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest

Now, trying to print the attestations results in errors.

> docker buildx imagetools inspect ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest --format "{{json .SBOM}}"

ERROR: failed to copy: httpReadSeeker: failed open: content at https://ghcr.io/v2/mkoepf/ghcrctl-test-with-sbom/manifests/sha256:c

> docker buildx imagetools inspect ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest --format "{{json .Provenance}}"

ERROR: failed to copy: httpReadSeeker: failed open: content at https://ghcr.io/v2/mkoepf/ghcrctl-test-with-sbom/manifests/sha256:cca2d3fbaac0f656634786844f9e002230d23100b84c4201c9f53c44392130bc not found: not found

Next, I delete the linux/arm64 manifest (whose version-id I get with ghcrctl versions).

Then, I delete the previously pulled image, again:

> docker rmi ghcr.io/mkoepf/ghcrctl-test-with-sbom

When I now try to pull, I get an error:

> docker pull ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest

latest: Pulling from mkoepf/ghcrctl-test-with-sbom
manifest unknown

Note, that this is different from trying to pull an image that never had an linux/arm64 manifest, e.g.:

> docker pull ghcr.io/mkoepf/ghcrctl-test-no-sbom:latest
latest: Pulling from mkoepf/ghcrctl-test-no-sbom
no matching manifest for linux/arm64/v8 in the manifest list entries

## Conclusion:

Delete parts of a graph will be a rare use case, if at all. It breaks the logic
of the image.

In all normal cases, the graph is actually the atom of ghcr operations. This is
how ghcrctl should also operate.