class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "10"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v10/grubby_v10_darwin_arm64.tar.gz"
      sha256 "047216275827b7ec2c3bf15f8c86ee6e140524f8d17bab1d9f0bfbdf5a709246"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v10/grubby_v10_darwin_amd64.tar.gz"
      sha256 "5cad9a8f68a7663f42af1b30fc0335c10ccce5c126053f172478b47f9ee82c0d"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v10/grubby_v10_linux_arm64.tar.gz"
      sha256 "0c755c69b8e9440da8c2fb7cd75e669dfe140c18541e8ff536db551ed02b863d"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v10/grubby_v10_linux_amd64.tar.gz"
      sha256 "ce7f536a30a381de2cc508fcba5d3b2eade09cdbd54508ba47c9cd792ab0266e"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
