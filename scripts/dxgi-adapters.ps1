# Outputs CSV lines: dxgi_index,high_part:low_part,adapter_name
# Used to align WMI GPUs with DXGI adapter order (Windows only).
$ErrorActionPreference = 'Stop'
Add-Type @"
using System;
using System.Runtime.InteropServices;
using System.Text;
public class DxgiEnum {
  [StructLayout(LayoutKind.Sequential)] public struct LUID { public uint LowPart; public int HighPart; }
  [StructLayout(LayoutKind.Sequential, CharSet=CharSet.Unicode)] public struct DXGI_ADAPTER_DESC1 {
    [MarshalAs(UnmanagedType.ByValTStr, SizeConst=128)] public string Description;
    public uint VendorId; public uint DeviceId; public uint SubSysId; public uint Revision;
    public IntPtr DedicatedVideoMemory; public IntPtr DedicatedSystemMemory; public IntPtr SharedSystemMemory;
    public LUID AdapterLuid; public uint Flags;
  }
  [Guid("790aabf0-8d54-4caf-9f1c-156e65f0c0e1")] [InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
  public interface IDXGIFactory1 {
    int EnumAdapters1(uint Adapter, out IntPtr ppAdapter);
  }
  [Guid("29038f61-3839-4626-91fd-086879011a05")] [InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
  public interface IDXGIAdapter1 {
    int GetDesc1(out DXGI_ADAPTER_DESC1 pDesc);
  }
  [DllImport("dxgi.dll", EntryPoint="CreateDXGIFactory1")] public static extern int CreateDXGIFactory1(ref Guid riid, out IntPtr ppFactory);
}
"@
$riid = [Guid]"790aabf0-8d54-4caf-9f1c-156e65f0c0e1"
$factoryPtr = [IntPtr]::Zero
[void][DxgiEnum]::CreateDXGIFactory1([ref]$riid, [ref]$factoryPtr)
$factory = [System.Runtime.InteropServices.Marshal]::GetObjectForIUnknown($factoryPtr)
$idx = 0
while ($true) {
  $adapterPtr = [IntPtr]::Zero
  $hr = $factory.EnumAdapters1([uint32]$idx, [ref]$adapterPtr)
  if ($hr -ne 0) { break }
  $adapter = [System.Runtime.InteropServices.Marshal]::GetObjectForIUnknown($adapterPtr)
  $desc = New-Object DxgiEnum+DXGI_ADAPTER_DESC1
  [void]$adapter.GetDesc1([ref]$desc)
  $luid = "{0}:{1}" -f $desc.AdapterLuid.HighPart, $desc.AdapterLuid.LowPart
  Write-Output ("{0},{1},{2}" -f $idx, $luid, $desc.Description.Trim())
  $idx++
}
