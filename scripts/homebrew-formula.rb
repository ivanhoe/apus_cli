# This file is a template. The release workflow replaces VERSION and SHA256
# values, then pushes the result to ivanhoe/homebrew-tap as Formula/apus.rb.
class Apus < Formula
  desc "CLI for integrating the Apus runtime into iOS projects"
  homepage "https://github.com/ivanhoe/apus_cli"
  version "VERSION"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/ivanhoe/apus_cli/releases/download/vVERSION/apus_vVERSION_darwin_arm64.tar.gz"
      sha256 "SHA256_ARM64"
    else
      url "https://github.com/ivanhoe/apus_cli/releases/download/vVERSION/apus_vVERSION_darwin_amd64.tar.gz"
      sha256 "SHA256_AMD64"
    end
  end

  def install
    bin.install "apus"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/apus --version")
  end
end
