//go:build amd64 && amd64.v3

#include "textflag.h"

// transformInputsAVX2 performs the two perspective-ordered halves of the BIG
// clipped-product transform. VPADDW preserves the reference's defined int16
// wrapping before signed clipping; every clipped product is below 2^16.
TEXT ·transformInputsAVX2(SB), NOSPLIT, $0-40
	MOVQ output+0(FP), DI
	MOVQ stmBase+8(FP), AX
	MOVQ stmThreats+16(FP), BX

	VPXOR Y4, Y4, Y4
	MOVL $255, R10
	VMOVD R10, X5
	VPBROADCASTW X5, Y5
	MOVL $2, R9
transform_perspective:
	LEAQ 1024(AX), CX
	LEAQ 1024(BX), DX
	MOVL $32, R8
transform_block:
	VMOVDQU (AX), Y0
	VPADDW (BX), Y0, Y0
	VPMAXSW Y4, Y0, Y0
	VPMINSW Y5, Y0, Y0

	VMOVDQU (CX), Y1
	VPADDW (DX), Y1, Y1
	VPMAXSW Y4, Y1, Y1
	VPMINSW Y5, Y1, Y1

	VPMULLW Y1, Y0, Y0
	VPSRLW $9, Y0, Y0
	VEXTRACTI128 $1, Y0, X1
	VPACKUSWB X1, X0, X0
	VMOVDQU X0, (DI)

	ADDQ $32, AX
	ADDQ $32, BX
	ADDQ $32, CX
	ADDQ $32, DX
	ADDQ $16, DI
	DECL R8
	JNZ transform_block

	DECL R9
	JZ transform_done
	MOVQ ntmBase+24(FP), AX
	MOVQ ntmThreats+32(FP), BX
	JMP transform_perspective
transform_done:
	VZEROUPPER
	RET
