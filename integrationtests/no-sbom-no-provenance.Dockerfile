FROM alpine:3.19
ARG GITHUB_REPOSITORY
LABEL org.opencontainers.image.source="https://github.com/${GITHUB_REPOSITORY}"
LABEL org.opencontainers.image.description="Test image without SBOM or provenance (multiarch)"
LABEL test.image.type="no-sbom-no-provenance-multiarch"
RUN echo "Test image 4 - no SBOM, no provenance, multiarch" > /test.txt
HEALTHCHECK --interval=30s --timeout=3s CMD cat /test.txt || exit 1
USER nobody
CMD ["cat", "/test.txt"]
