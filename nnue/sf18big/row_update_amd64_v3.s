//go:build amd64 && amd64.v3

#include "textflag.h"

TEXT ·addBaseRowAVX2(SB), NOSPLIT, $0-32
	MOVQ values+0(FP), AX
	MOVQ weights+8(FP), BX
	MOVQ psqt+16(FP), CX
	MOVQ psqtWeights+24(FP), DX
	MOVL $64, R8
add_base_loop:
	VMOVDQU (AX), Y0
	VPADDW (BX), Y0, Y0
	VMOVDQU Y0, (AX)
	ADDQ $32, AX
	ADDQ $32, BX
	DECL R8
	JNZ add_base_loop
	VMOVDQU (CX), Y0
	VPADDD (DX), Y0, Y0
	VMOVDQU Y0, (CX)
	VZEROUPPER
	RET

TEXT ·subBaseRowAVX2(SB), NOSPLIT, $0-32
	MOVQ values+0(FP), AX
	MOVQ weights+8(FP), BX
	MOVQ psqt+16(FP), CX
	MOVQ psqtWeights+24(FP), DX
	MOVL $64, R8
sub_base_loop:
	VMOVDQU (AX), Y0
	VPSUBW (BX), Y0, Y0
	VMOVDQU Y0, (AX)
	ADDQ $32, AX
	ADDQ $32, BX
	DECL R8
	JNZ sub_base_loop
	VMOVDQU (CX), Y0
	VPSUBD (DX), Y0, Y0
	VMOVDQU Y0, (CX)
	VZEROUPPER
	RET

TEXT ·addThreatRowAVX2(SB), NOSPLIT, $0-32
	MOVQ values+0(FP), AX
	MOVQ weights+8(FP), BX
	MOVQ psqt+16(FP), CX
	MOVQ psqtWeights+24(FP), DX
	MOVL $64, R8
add_threat_loop:
	VMOVDQU (BX), X1
	VPMOVSXBW X1, Y1
	VMOVDQU (AX), Y0
	VPADDW Y1, Y0, Y0
	VMOVDQU Y0, (AX)
	ADDQ $32, AX
	ADDQ $16, BX
	DECL R8
	JNZ add_threat_loop
	VMOVDQU (CX), Y0
	VPADDD (DX), Y0, Y0
	VMOVDQU Y0, (CX)
	VZEROUPPER
	RET

TEXT ·subThreatRowAVX2(SB), NOSPLIT, $0-32
	MOVQ values+0(FP), AX
	MOVQ weights+8(FP), BX
	MOVQ psqt+16(FP), CX
	MOVQ psqtWeights+24(FP), DX
	MOVL $64, R8
sub_threat_loop:
	VMOVDQU (BX), X1
	VPMOVSXBW X1, Y1
	VMOVDQU (AX), Y0
	VPSUBW Y1, Y0, Y0
	VMOVDQU Y0, (AX)
	ADDQ $32, AX
	ADDQ $16, BX
	DECL R8
	JNZ sub_threat_loop
	VMOVDQU (CX), Y0
	VPSUBD (DX), Y0, Y0
	VMOVDQU Y0, (CX)
	VZEROUPPER
	RET
