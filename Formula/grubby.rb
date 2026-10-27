class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "17"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v17/grubby_v17_darwin_arm64.tar.gz"
      sha256 "1b7223b96fec0cfb18ca2f12bc4a2b750a364594c48ed4105f3d3c8b9a132933"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v17/grubby_v17_darwin_amd64.tar.gz"
      sha256 "db1729ba1eed0e6ed3068da502c989a3b10a0d49108adb5c4bf748fc5922ccf5"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v17/grubby_v17_linux_arm64.tar.gz"
      sha256 "5ce5fc1d7e4259b76e3856b9a35adedf55f4a5fb942cfa45bd771793f2ceee6e"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v17/grubby_v17_linux_amd64.tar.gz"
      sha256 "91349ffd3ff68ce820f88f674eedd6e5ea59bc3b7f2f4fd3de041d1dbe64a94f"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
