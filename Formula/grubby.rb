class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "12"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v12/grubby_v12_darwin_arm64.tar.gz"
      sha256 "7208d8209ba65a1af2958441440035aa2bd339c3263deadf2e5b4874f5fb67f6"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v12/grubby_v12_darwin_amd64.tar.gz"
      sha256 "b9789b852c0b4d15092ddda9e70b30ddb7a4076d8bc5f0101197a2cbc3fb35db"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v12/grubby_v12_linux_arm64.tar.gz"
      sha256 "507fae2bcdc3d1383dd41975dac4a07f42fd76f260992ef804a97a682c3c5ec6"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v12/grubby_v12_linux_amd64.tar.gz"
      sha256 "98be64d07c91708a4b01c796e35449daa523b47030db3765aa849d1062597419"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
