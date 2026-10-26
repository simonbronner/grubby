class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "11"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v11/grubby_v11_darwin_arm64.tar.gz"
      sha256 "047e2d59abf530c863ddb47bd2765c16d5780d3d6f58550fb52d6a083c6d8847"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v11/grubby_v11_darwin_amd64.tar.gz"
      sha256 "7bc0f77bbd068a30f7dfcf4e58b02cadffe0de36356142b17da7a6c125aa9e32"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v11/grubby_v11_linux_arm64.tar.gz"
      sha256 "2cdab2daa1f0316718213287035a47d40a7a2a01c953002491c79667aa8bf573"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v11/grubby_v11_linux_amd64.tar.gz"
      sha256 "cfd3d694e256a66a6575444ba98d3d7b7ad9bb315c23106867834c185f5efda7"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
