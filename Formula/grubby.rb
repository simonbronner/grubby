class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "11"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v11/grubby_v11_darwin_arm64.tar.gz"
      sha256 "ec428f99e06b920b731bf5f5a665830bacee2bd3b6a9437c7c977f0f91db8dc7"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v11/grubby_v11_darwin_amd64.tar.gz"
      sha256 "9bca9261fa12fb9b3e0ca0e3bb6246288cdaff9dbcadb16d1e042c0095cd0786"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v11/grubby_v11_linux_arm64.tar.gz"
      sha256 "b4a378747b90ecd21e066802b6f1261dab203959def4b5a0f893f039c8cac95a"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v11/grubby_v11_linux_amd64.tar.gz"
      sha256 "256272f89df22cf50d94b88807969ec526d7cb70afaea6657ace6212dbd38414"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
