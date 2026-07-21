import AssetFunctionIcon from "../functions/AssetFunctionIcon";
import { functionIconAssetUrl } from "../../icons/functionIconAssets";

const Z21_ICON_SRC = functionIconAssetUrl("z21");

interface Z21IconProps {
  size?: number;
  className?: string;
}

/** Roco Z21 handset icon from `web/src/icons/png/z21.png`. */
export default function Z21Icon({ size = 24, className }: Z21IconProps) {
  if (!Z21_ICON_SRC) {
    return null;
  }
  return (
    <AssetFunctionIcon src={Z21_ICON_SRC} size={size} className={className} />
  );
}
