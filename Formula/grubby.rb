class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "18"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v18/grubby_v18_darwin_arm64.tar.gz"
      sha256 "0b1943d9efbac0d6ef9544379a7562bc2f647383115d0d5ad7c9b88427bc6629"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v18/grubby_v18_darwin_amd64.tar.gz"
      sha256 "151cbe256dedffda3654772d0c2fa82c7c70ca3ed4a65734bdff577cba101c4d"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v18/grubby_v18_linux_arm64.tar.gz"
      sha256 "b2680f51c04d97da54afa35a1329694a1356f67957196a1735918407ce0ea535"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v18/grubby_v18_linux_amd64.tar.gz"
      sha256 "d907ecd6f28e604bc88e7e0e745133b453b0a35ed268b4dc9fc27f17131469de"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
