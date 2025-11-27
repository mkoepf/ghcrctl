FROM scratch
ARG GITHUB_REPOSITORY
LABEL org.opencontainers.image.source="https://github.com/${GITHUB_REPOSITORY}"
LABEL org.opencontainers.image.description="Test image with SBOM and provenance (multiarch)"
LABEL test.image.type="with-sbom-with-provenance-multiarch"
USER nobody
