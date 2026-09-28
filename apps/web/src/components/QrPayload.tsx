import { QRCodeSVG } from "qrcode.react";

type QrPayloadProps = {
  value: string;
};

export default function QrPayload({ value }: QrPayloadProps) {
  return (
    <div className="qr">
      <QRCodeSVG
        value={value}
        size={220}
        level="M"
        includeMargin
        title="QR code do pagamento"
        bgColor="#fffaf3"
        fgColor="#241910"
      />
    </div>
  );
}
