# typed: false
# frozen_string_literal: true

class Tce < Formula
  desc "Terminal Coding Assistant — CLI agent for any OpenAI-compatible LLM"
  homepage "https://github.com/talen400/tce"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/talen400/tce/releases/latest/download/tce_darwin_arm64"
      sha256 "SET_ME" # Replace with actual checksum from release
    else
      url "https://github.com/talen400/tce/releases/latest/download/tce_darwin_amd64"
      sha256 "SET_ME"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/talen400/tce/releases/latest/download/tce_linux_arm64"
      sha256 "SET_ME"
    else
      url "https://github.com/talen400/tce/releases/latest/download/tce_linux_amd64"
      sha256 "SET_ME"
    end
  end

  def install
    bin.install "tce"
  end

  test do
    assert_match "tce v", shell_output("#{bin}/tce --version")
  end
end
