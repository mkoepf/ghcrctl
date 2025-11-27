FROM alpine:3.22
ARG GITHUB_REPOSITORY
LABEL org.opencontainers.image.source="https://github.com/${GITHUB_REPOSITORY}"
LABEL org.opencontainers.image.description="Test image without SBOM or provenance"
LABEL test.image.type="no-sbom-no-provenance-single"
RUN echo "Test image 1 - no SBOM, no provenance" > /test.txt
HEALTHCHECK --interval=30s --timeout=3s CMD cat /test.txt || exit 1
USER nobody
CMD ["cat", "/test.txt"]
